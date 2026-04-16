import asyncHandler from 'express-async-handler';
import Document from '../models/Document.js';
import Folder from '../models/Folder.js';
import Team from '../models/Team.js';
import { createNotifications, emitTeamUpdate } from '../utils/notificationService.js';
import { upload, cloudinary, CLOUDINARY_ENABLED } from '../config/cloudinary.js';
import path from 'path';
import fs from 'fs';

// ---------- helpers ----------

/**
 * Extract Cloudinary public_id from a full CDN URL.
 * e.g. https://res.cloudinary.com/{cloud}/raw/upload/v123/aurora-docs/1234-resume
 *   → "aurora-docs/1234-resume"
 */
const extractCloudinaryPublicId = (url) => {
    try {
        const match = url.match(/\/upload\/(?:v\d+\/)?(.+)$/);
        if (match) return match[1].replace(/\.[^/.]+$/, '');
    } catch (_) {}
    return null;
};

/**
 * Map a file extension to the Cloudinary resource_type needed for URL delivery.
 * PDFs and office docs must use 'raw' — accessing them via 'image' returns 401.
 */
const IMAGE_EXTS = ['jpg','jpeg','png','gif','webp','svg','bmp','ico','tiff','avif'];
const VIDEO_EXTS = ['mp4','avi','mov','webm','mkv','flv','wmv','m4v'];
const getCloudinaryResourceType = (filename) => {
    const ext = (filename || '').split('.').pop().toLowerCase();
    if (IMAGE_EXTS.includes(ext)) return 'image';
    if (VIDEO_EXTS.includes(ext)) return 'video';
    return 'raw';  // PDF, DOCX, XLSX, ZIP, etc.
};

/**
 * Delete a file from whichever storage it lives in.
 *   doc.cloudinaryId  → use stored public_id (preferred)
 *   doc.url = http(s) → extract public_id from URL (fallback for older records)
 *   doc.url = /uploads/... → delete from local disk
 */
const deleteStoredFile = async (doc) => {
    const isCloudinaryUrl = doc.url && doc.url.startsWith('http');

    if (isCloudinaryUrl && CLOUDINARY_ENABLED) {
        const publicId     = doc.cloudinaryId || extractCloudinaryPublicId(doc.url);
        const resourceType = getCloudinaryResourceType(doc.name || doc.url || '');
        if (publicId) {
            try {
                const result = await cloudinary.uploader.destroy(publicId, { resource_type: resourceType });
                console.log(`[Cloudinary] deleted public_id="${publicId}" type=${resourceType} result=${result.result}`);
            } catch (err) {
                console.error('[Cloudinary] delete error:', err.message);
            }
        } else {
            console.warn('[Cloudinary] could not determine public_id for URL:', doc.url);
        }
    } else if (!isCloudinaryUrl && doc.url && doc.url.startsWith('/uploads/')) {
        try {
            const filePath = path.join(path.resolve(), doc.url.replace(/^\//, ''));
            if (fs.existsSync(filePath)) {
                fs.unlinkSync(filePath);
                console.log(`[Local] deleted: ${filePath}`);
            }
        } catch (err) {
            console.error('[Local] delete error:', err.message);
        }
    }
};

// ---------- Controllers ----------

// @desc    Get documents and folders for a specific team
// @route   GET /api/documents?teamId=...&folderId=...
// @access  Private
const getDocuments = asyncHandler(async (req, res) => {
    const { teamId, folderId } = req.query;

    if (!teamId) {
        res.status(400);
        throw new Error('Team ID is required');
    }

    const team = await Team.findById(teamId);
    if (!team) {
        res.status(404);
        throw new Error('Team not found');
    }

    const userId = req.user.id.toString();
    const isMasterAdmin = req.user.role === 'master_admin';
    const isMember = isMasterAdmin
        || team.members.some(m => m && m.toString() === userId)
        || team.owner.toString() === userId;

    if (!isMember) {
        res.status(403);
        throw new Error('Not authorized to access this team files');
    }

    const currentFolderId = folderId && folderId !== 'null' ? folderId : null;

    const [folders, documents] = await Promise.all([
        Folder.find({ team: teamId, parentFolder: currentFolderId }).populate('createdBy', 'name'),
        Document.find({ team: teamId, folder: currentFolderId }).populate('uploadedBy', 'name'),
    ]);

    let currentFolder = null;
    if (currentFolderId) currentFolder = await Folder.findById(currentFolderId);

    res.json({ folders, documents, currentFolder });
});

// @desc    Upload file (Cloudinary if configured, local disk otherwise)
// @route   POST /api/documents/upload
// @access  Private
const uploadDocument = asyncHandler(async (req, res) => {
    await new Promise((resolve, reject) => {
        upload(req, res, (err) => {
            if (err) return reject(err);
            resolve();
        });
    }).catch((err) => {
        res.status(400);
        throw new Error(err.message || 'File upload failed');
    });

    const { teamId, folderId } = req.body;

    if (!req.file) {
        res.status(400);
        throw new Error('No file uploaded');
    }

    const team = await Team.findById(teamId);
    if (!team) {
        await deleteStoredFile({ cloudinaryId: req.file.filename, url: req.file.path });
        res.status(404);
        throw new Error('Team not found');
    }

    const userId = req.user.id.toString();
    const isMasterAdmin = req.user.role === 'master_admin';
    const isMember = isMasterAdmin
        || team.members.some(m => m && m.toString() === userId)
        || team.owner.toString() === userId;

    if (!isMember) {
        await deleteStoredFile({ cloudinaryId: req.file.filename, url: req.file.path });
        res.status(403);
        throw new Error('Not authorized to upload to this team');
    }

    // Cloudinary mode → req.file.path = https CDN URL, req.file.filename = public_id
    // Local mode      → req.file.path = absolute disk path, build relative URL
    let storedUrl;
    let cloudinaryId = null;

    if (CLOUDINARY_ENABLED && req.file.path && req.file.path.startsWith('http')) {
        storedUrl    = req.file.path;
        cloudinaryId = req.file.filename;
    } else {
        storedUrl = `/uploads/${req.file.filename}`;
    }

    const document = await Document.create({
        team: teamId,
        folder: folderId && folderId !== 'null' ? folderId : null,
        uploadedBy: req.user.id,
        user: req.user.id,
        name: req.file.originalname,
        type: req.file.originalname.split('.').pop().toLowerCase(),
        size: req.file.size
            ? `${(req.file.size / (1024 * 1024)).toFixed(2)} MB`
            : 'Unknown',
        url: storedUrl,
        cloudinaryId,
    });

    const io = req.app.get('socketio');
    const recipientIds = [...team.members, team.owner]
        .filter(id => id != null)
        .map(id => id.toString())
        .filter(id => id !== req.user.id.toString());

    if (recipientIds.length > 0) {
        await createNotifications(recipientIds, {
            title: 'New Document Uploaded',
            description: `"${document.name}" has been uploaded to the team library`,
            type: 'document_shared',
            sender: req.user.id,
            link: '/document-share',
        }, io);
    }

    emitTeamUpdate(io, teamId, 'DOCUMENT_UPLOAD');

    const populatedDoc = await Document.findById(document._id).populate('uploadedBy', 'name');
    res.status(201).json(populatedDoc);
});

// @desc    Create Folder
// @route   POST /api/documents/folder
// @access  Private
const createFolder = asyncHandler(async (req, res) => {
    const { name, teamId, parentFolderId } = req.body;
    if (!name || !teamId) {
        res.status(400);
        throw new Error('Name and Team ID required');
    }

    const folder = await Folder.create({
        name,
        team: teamId,
        parentFolder: parentFolderId || null,
        createdBy: req.user.id,
    });

    const io = req.app.get('socketio');
    emitTeamUpdate(io, teamId, 'FOLDER_CREATE');
    res.status(201).json(folder);
});

// @desc    Rename Folder (Head/Admin only)
// @route   PUT /api/documents/folder/:id
// @access  Private
const renameFolder = asyncHandler(async (req, res) => {
    const { name } = req.body;
    const folder = await Folder.findById(req.params.id);
    if (!folder) {
        res.status(404);
        throw new Error('Folder not found');
    }

    const isMasterAdmin = req.user.role === 'master_admin';
    const isHead = isMasterAdmin || req.user.role === 'team_head' || req.user.role === 'admin';
    if (!isHead) {
        res.status(403);
        throw new Error('Only Team Heads can rename folders');
    }

    folder.name = name || folder.name;
    await folder.save();

    const io = req.app.get('socketio');
    emitTeamUpdate(io, folder.team, 'FOLDER_RENAME');
    res.json(folder);
});

// @desc    Delete Folder (Head/Admin only) — recursive
// @route   DELETE /api/documents/folder/:id
// @access  Private
const deleteFolder = asyncHandler(async (req, res) => {
    const folder = await Folder.findById(req.params.id);
    if (!folder) {
        res.status(404);
        throw new Error('Folder not found');
    }

    const isMasterAdmin = req.user.role === 'master_admin';
    const isHead = isMasterAdmin || req.user.role === 'team_head' || req.user.role === 'admin';
    if (!isHead) {
        res.status(403);
        throw new Error('Only Team Heads can delete folders');
    }

    const teamId = folder.team;

    const deleteFolderRecursive = async (folderId) => {
        const subfolders = await Folder.find({ parentFolder: folderId });
        for (const sub of subfolders) await deleteFolderRecursive(sub._id);

        const docs = await Document.find({ folder: folderId });
        for (const doc of docs) {
            await deleteStoredFile(doc);
            await doc.deleteOne();
        }
        await Folder.findByIdAndDelete(folderId);
    };

    const io = req.app.get('socketio');
    const team = await Team.findById(folder.team);
    if (team) {
        const recipientIds = [...team.members, team.owner]
            .filter(id => id != null)
            .map(id => id.toString())
            .filter(id => id !== req.user.id.toString());

        if (recipientIds.length > 0) {
            await createNotifications(recipientIds, {
                title: 'Folder Removed',
                description: `${req.user.name} deleted the folder "${folder.name}" and its contents`,
                type: 'document_shared',
                sender: req.user.id,
                link: '/document-share',
            }, io);
        }
    }

    await deleteFolderRecursive(folder._id);
    emitTeamUpdate(io, teamId, 'FOLDER_DELETE');
    res.json({ message: 'Folder deleted' });
});

// @desc    Download document (auth-gated)
// @route   GET /api/documents/download/:id
// @access  Private
//
// For Cloudinary files: verify auth, return JSON { downloadUrl } with fl_attachment.
//   The browser fetches directly from CDN — no server-side proxy needed.
// For local files: stream from disk with Content-Disposition: attachment.
const downloadDocument = asyncHandler(async (req, res) => {
    const document = await Document.findById(req.params.id);
    if (!document) {
        res.status(404);
        throw new Error('Document not found');
    }

    const team = await Team.findById(document.team);
    if (!team) {
        res.status(404);
        throw new Error('Team not found');
    }

    const userId      = req.user.id.toString();
    const isMasterAdmin = req.user.role === 'master_admin';
    const isHead      = isMasterAdmin || req.user.role === 'team_head';
    const isMember    = isMasterAdmin
        || team.members.some(m => m && m.toString() === userId)
        || team.owner.toString() === userId;

    if (!isMember) {
        res.status(403);
        throw new Error('Not authorized to download this file');
    }

    if (!document.isDownloadable && !isHead) {
        res.status(403);
        throw new Error('Downloads have been restricted for this file by the team head');
    }

    const fileName  = document.name;
    const storedUrl = document.url || '';

    console.log(`[Download] doc=${document._id}  url=${storedUrl}`);

    // ── CASE 1: Absolute URL (Cloudinary CDN) ────────────────────────────────
    if (storedUrl.startsWith('http')) {
        const isLocalhost = /https?:\/\/(localhost|127\.0\.0\.1)(:\d+)?/.test(storedUrl);

        if (isLocalhost) {
            // Localhost URL stored by mistake — strip host, serve from disk
            const urlPath  = storedUrl.replace(/^https?:\/\/[^/]+/, '');
            const diskPath = path.join(path.resolve(), urlPath.replace(/^\//, ''));
            if (fs.existsSync(diskPath)) {
                const enc = encodeURIComponent(fileName);
                res.setHeader('Content-Disposition', `attachment; filename="${fileName}"; filename*=UTF-8''${enc}`);
                res.setHeader('Content-Type', 'application/octet-stream');
                res.setHeader('Access-Control-Expose-Headers', 'Content-Disposition');
                fs.createReadStream(diskPath).pipe(res);
                // Self-heal DB record
                Document.findByIdAndUpdate(document._id, { url: urlPath, cloudinaryId: null }).catch(() => {});
            } else {
                res.status(404).json({ message: 'File not found on disk. Please re-upload.' });
            }
            return;
        }

        // Real Cloudinary URL.
        // Build the download URL via the Cloudinary SDK so the resource_type is always
        // correct (image/video/raw). Accessing a raw PDF via /image/upload/ gives 401.
        const publicId = document.cloudinaryId || extractCloudinaryPublicId(storedUrl);
        const resourceType = getCloudinaryResourceType(fileName);

        let downloadUrl;
        if (publicId && CLOUDINARY_ENABLED) {
            // SDK-generated URL — guaranteed correct resource_type + fl_attachment
            downloadUrl = cloudinary.url(publicId, {
                resource_type: resourceType,
                secure: true,
                flags: 'attachment',
            });
            console.log(`[Download] SDK URL (${resourceType}): ${downloadUrl}`);
        } else {
            // Fallback: patch the stored URL resource type from the extension
            const correctType = resourceType; // 'image' | 'video' | 'raw'
            downloadUrl = storedUrl
                .replace(/\/(image|video|raw)\/upload\//, `/${correctType}/upload/`)
                .replace('/upload/', '/upload/fl_attachment/');
            console.log(`[Download] Patched URL: ${downloadUrl}`);
        }

        return res.json({ downloadUrl, fileName });
    }

    // ── CASE 2: Local disk (/uploads/...) ────────────────────────────────────
    const filePath = path.join(path.resolve(), storedUrl.replace(/^\//, ''));
    if (!fs.existsSync(filePath)) {
        return res.status(404).json({
            message: 'File not found on server. It may have been lost after a restart. Please re-upload.',
        });
    }

    const enc = encodeURIComponent(fileName);
    res.setHeader('Content-Disposition', `attachment; filename="${fileName}"; filename*=UTF-8''${enc}`);
    res.setHeader('Content-Type', 'application/octet-stream');
    res.setHeader('Access-Control-Expose-Headers', 'Content-Disposition');

    const fileStream = fs.createReadStream(filePath);
    fileStream.on('error', (e) => {
        console.error('[Download] stream error:', e.message);
        if (!res.headersSent) res.status(500).json({ message: 'Error reading file' });
    });
    fileStream.pipe(res);
});

// @desc    Delete document
// @route   DELETE /api/documents/:id
// @access  Private (uploader, head, master_admin)
const deleteDocument = asyncHandler(async (req, res) => {
    const document = await Document.findById(req.params.id);
    if (!document) {
        res.status(404);
        throw new Error('Document not found');
    }

    const isUploader  = document.uploadedBy.toString() === req.user.id.toString();
    const isMasterAdmin = req.user.role === 'master_admin';
    const isHead      = isMasterAdmin || req.user.role === 'team_head' || req.user.role === 'admin';

    if (!isUploader && !isHead) {
        res.status(403);
        throw new Error('Not authorized to delete this file');
    }

    const teamId = document.team;
    await deleteStoredFile(document);
    await document.deleteOne();

    const team = await Team.findById(teamId);
    const io   = req.app.get('socketio');
    if (team) {
        const recipientIds = [...team.members, team.owner]
            .filter(id => id != null)
            .map(id => id.toString())
            .filter(id => id !== req.user.id.toString());

        if (recipientIds.length > 0) {
            await createNotifications(recipientIds, {
                title: 'Document Removed',
                description: `${req.user.name} deleted "${document.name}"`,
                type: 'document_shared',
                sender: req.user.id,
                link: '/document-share',
            }, io);
        }
    }

    emitTeamUpdate(io, teamId, 'DOCUMENT_DELETE');
    res.json({ id: req.params.id });
});

// @desc    Toggle isDownloadable flag
// @route   PATCH /api/documents/:id/downloadable
// @access  Private (uploader, head, master_admin)
const toggleDownloadable = asyncHandler(async (req, res) => {
    const document = await Document.findById(req.params.id);
    if (!document) {
        res.status(404);
        throw new Error('Document not found');
    }

    const isMasterAdmin = req.user.role === 'master_admin';
    const isHead        = isMasterAdmin || req.user.role === 'team_head';
    const isUploader    = document.uploadedBy.toString() === req.user.id.toString();

    if (!isHead && !isUploader) {
        res.status(403);
        throw new Error('Only Team Heads, Master Admins, or the uploader can change download permissions');
    }

    document.isDownloadable = !document.isDownloadable;
    await document.save();

    const io = req.app.get('socketio');
    emitTeamUpdate(io, document.team, 'DOCUMENT_UPDATE');
    res.json({ _id: document._id, isDownloadable: document.isDownloadable });
});

export {
    getDocuments,
    uploadDocument,
    downloadDocument,
    deleteDocument,
    createFolder,
    renameFolder,
    deleteFolder,
    toggleDownloadable,
};
