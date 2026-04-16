import asyncHandler from 'express-async-handler';
import Document from '../models/Document.js';
import Folder from '../models/Folder.js';
import Team from '../models/Team.js';
import { createNotifications, emitTeamUpdate } from '../utils/notificationService.js';
import { upload, cloudinary, CLOUDINARY_ENABLED } from '../config/cloudinary.js';
import path from 'path';
import fs from 'fs';
import https from 'https';
import http from 'http';

// ---------- helpers ----------

/**
 * Delete a file from whichever storage it lives in.
 * doc.cloudinaryId → Cloudinary
 * doc.url starts with /uploads/ → local disk
 */
const deleteStoredFile = async (doc) => {
    if (doc.cloudinaryId && CLOUDINARY_ENABLED) {
        try {
            await cloudinary.uploader.destroy(doc.cloudinaryId, { resource_type: 'auto' });
        } catch (err) {
            console.error('[Cloudinary] delete error:', err.message);
        }
    } else if (doc.url && doc.url.startsWith('/uploads/')) {
        try {
            const filePath = path.join(path.resolve(), doc.url.replace(/^\//, ''));
            if (fs.existsSync(filePath)) fs.unlinkSync(filePath);
        } catch (err) {
            console.error('[Local] delete error:', err.message);
        }
    }
};

/**
 * Pipe a remote HTTPS/HTTP URL to the Express response.
 * Used to proxy Cloudinary files back to the client so that
 * auth is enforced on our side and fetch() gets a normal streamable response.
 */
const proxyRemoteFile = (remoteUrl, res, filename) => {
    return new Promise((resolve, reject) => {
        const proto = remoteUrl.startsWith('https') ? https : http;
        proto.get(remoteUrl, (remoteRes) => {
            if (remoteRes.statusCode !== 200) {
                reject(new Error(`Remote file returned ${remoteRes.statusCode}`));
                return;
            }
            const contentType = remoteRes.headers['content-type'] || 'application/octet-stream';
            const encodedName = encodeURIComponent(filename);
            res.setHeader('Content-Type', contentType);
            res.setHeader('Content-Disposition', `attachment; filename="${filename}"; filename*=UTF-8''${encodedName}`);
            res.setHeader('Access-Control-Expose-Headers', 'Content-Disposition');
            if (remoteRes.headers['content-length']) {
                res.setHeader('Content-Length', remoteRes.headers['content-length']);
            }
            remoteRes.pipe(res);
            remoteRes.on('end', resolve);
            remoteRes.on('error', reject);
        }).on('error', reject);
    });
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

    // Determine stored URL:
    //   Cloudinary mode → req.file.path is the https://res.cloudinary.com/... CDN URL
    //   Local mode      → req.file.path is the absolute disk path; build relative URL
    let storedUrl;
    let cloudinaryId = null;

    if (CLOUDINARY_ENABLED && req.file.path && req.file.path.startsWith('http')) {
        storedUrl = req.file.path;          // Cloudinary CDN URL
        cloudinaryId = req.file.filename;   // Cloudinary public_id
    } else {
        // Local disk — build a relative URL the static server can serve
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

// @desc    Download document — auth-gated, streams file to client
// @route   GET /api/documents/download/:id
// @access  Private
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

    const userId = req.user.id.toString();
    const isMasterAdmin = req.user.role === 'master_admin';
    const isHead = isMasterAdmin || req.user.role === 'team_head';
    const isMember = isMasterAdmin
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

    const fileName = document.name;

    // ── CASE 1: Cloudinary-hosted (URL starts with http) ────────────────────
    if (document.url && document.url.startsWith('http')) {
        // Proxy the file through our server so the client just gets a plain stream.
        // This avoids the opaque-response problem that breaks fetch() blob downloads.
        try {
            await proxyRemoteFile(document.url, res, fileName);
        } catch (err) {
            console.error('[Download] Cloudinary proxy error:', err.message);
            if (!res.headersSent) {
                res.status(502);
                res.json({ message: 'Could not retrieve file from cloud storage' });
            }
        }
        return;
    }

    // ── CASE 2: Local disk ───────────────────────────────────────────────────
    const filePath = path.join(path.resolve(), document.url.replace(/^\//, ''));
    if (!fs.existsSync(filePath)) {
        res.status(404);
        throw new Error('File not found on server — it may have been lost after a server restart');
    }

    const encodedName = encodeURIComponent(fileName);
    res.setHeader('Content-Disposition', `attachment; filename="${fileName}"; filename*=UTF-8''${encodedName}`);
    res.setHeader('Content-Type', 'application/octet-stream');
    res.setHeader('Access-Control-Expose-Headers', 'Content-Disposition');

    const fileStream = fs.createReadStream(filePath);
    fileStream.on('error', (err) => {
        console.error('[Download] Read stream error:', err.message);
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

    const isUploader = document.uploadedBy.toString() === req.user.id.toString();
    const isMasterAdmin = req.user.role === 'master_admin';
    const isHead = isMasterAdmin || req.user.role === 'team_head' || req.user.role === 'admin';

    if (!isUploader && !isHead) {
        res.status(403);
        throw new Error('Not authorized to delete this file');
    }

    const teamId = document.team;
    await deleteStoredFile(document);
    await document.deleteOne();

    const team = await Team.findById(teamId);
    const io = req.app.get('socketio');
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
    const isHead = isMasterAdmin || req.user.role === 'team_head';
    const isUploader = document.uploadedBy.toString() === req.user.id.toString();

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
