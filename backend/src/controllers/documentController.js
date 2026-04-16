import asyncHandler from 'express-async-handler';
import Document from '../models/Document.js';
import Folder from '../models/Folder.js';
import Team from '../models/Team.js';
import { createNotifications, emitTeamUpdate } from '../utils/notificationService.js';
import multer from 'multer';
import path from 'path';
import fs from 'fs';

// --- Multer Utils ---
const storage = multer.diskStorage({
    destination(req, file, cb) {
        cb(null, 'uploads/');
    },
    filename(req, file, cb) {
        // Unique filename: Date + Original Name
        cb(null, `${Date.now()}-${file.originalname}`);
    },
});

const upload = multer({
    storage,
    limits: { fileSize: 50 * 1024 * 1024 }, // 50MB limit
}).single('file'); // 'file' is the form field name

// --- Controllers ---

// @desc    Get documents and folders for a specific team (and optional folder level)
// @route   GET /api/documents?teamId=...&folderId=...
// @access  Private
const getDocuments = asyncHandler(async (req, res) => {
    const { teamId, folderId } = req.query;

    if (!teamId) {
        res.status(400);
        throw new Error('Team ID is required');
    }

    // Verify membership
    const team = await Team.findById(teamId);
    if (!team) {
        res.status(404);
        throw new Error('Team not found');
    }

    // Check if user is member (or owner) OR Master Admin
    const userId = req.user.id.toString();
    const isMasterAdmin = req.user.role === 'master_admin';
    const isMember = isMasterAdmin || team.members.some(m => m && m.toString() === userId) || team.owner.toString() === userId;
    
    if (!isMember) {
        res.status(401);
        throw new Error('Not authorized to access this team files');
    }

    const currentFolderId = folderId && folderId !== 'null' ? folderId : null;

    // Fetch Folders
    const folders = await Folder.find({
        team: teamId,
        parentFolder: currentFolderId
    }).populate('createdBy', 'name');

    // Fetch Documents
    const documents = await Document.find({
        team: teamId,
        folder: currentFolderId
    }).populate('uploadedBy', 'name');

    // Get current folder details for breadcrumb if needed
    let currentFolder = null;
    if (currentFolderId) {
        currentFolder = await Folder.findById(currentFolderId);
    }

    res.json({ folders, documents, currentFolder });
});

// @desc    Upload file
// @route   POST /api/documents/upload
// @access  Private
const uploadDocument = asyncHandler(async (req, res) => {
    // Wrap multer in a Promise so asyncHandler properly catches errors
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

    // Validate Team
    const team = await Team.findById(teamId);
    if (!team) {
        // Cleanup file if error
        if (req.file) {
            try { fs.unlinkSync(req.file.path); } catch (_) { }
        }
        res.status(404);
        throw new Error('Team not found');
    }

    // Verify the uploader is a team member or owner OR Master Admin
    const userId = req.user.id.toString();
    const isMasterAdmin = req.user.role === 'master_admin';
    const isMember = isMasterAdmin || team.members.some(m => m && m.toString() === userId) || team.owner.toString() === userId;

    if (!isMember) {
        if (req.file) {
            try { fs.unlinkSync(req.file.path); } catch (_) { }
        }
        res.status(403);
        throw new Error('Not authorized to upload to this team');
    }

    const document = await Document.create({
        team: teamId,
        folder: folderId && folderId !== 'null' ? folderId : null,
        uploadedBy: req.user.id,
        user: req.user.id, // Legacy field, kept for safety
        name: req.file.originalname,
        type: req.file.originalname.split('.').pop(),
        size: (req.file.size / (1024 * 1024)).toFixed(2) + ' MB',
        url: `/uploads/${req.file.filename}`, // Relative URL
    });

    // Trigger Notification for the team
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
            link: '/document-share'
        }, io);
    }
    
    // Real-time Update
    emitTeamUpdate(io, teamId, 'DOCUMENT_UPLOAD');

    // Return full doc with populated user
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
        createdBy: req.user.id
    });

    const io = req.app.get('socketio');
    emitTeamUpdate(io, teamId, 'FOLDER_CREATE');

    res.status(201).json(folder);
});

// @desc    Rename Folder (Admin Only)
// @route   PUT /api/documents/folder/:id
// @access  Private (Admin/Head)
const renameFolder = asyncHandler(async (req, res) => {
    const { name } = req.body;
    const folder = await Folder.findById(req.params.id);

    if (!folder) {
        res.status(404);
        throw new Error('Folder not found');
    }

    // Check Admin/Head permission OR Master Admin
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

// @desc    Delete Folder (Admin Only)
// @route   DELETE /api/documents/folder/:id
// @access  Private (Admin/Head)
const deleteFolder = asyncHandler(async (req, res) => {
    const folder = await Folder.findById(req.params.id);

    if (!folder) {
        res.status(404);
        throw new Error('Folder not found');
    }

    // Check Admin/Head permission OR Master Admin
    const isMasterAdmin = req.user.role === 'master_admin';
    const isHead = isMasterAdmin || req.user.role === 'team_head' || req.user.role === 'admin';
    if (!isHead) {
        res.status(403);
        throw new Error('Only Team Heads can delete folders');
    }

    const teamId = folder.team;

    // Recursive helper to delete a folder and all its contents
    const deleteFolderRecursive = async (folderId) => {
        // 1. Find and recursively delete all subfolders
        const subfolders = await Folder.find({ parentFolder: folderId });
        for (const subfolder of subfolders) {
            await deleteFolderRecursive(subfolder._id);
        }

        // 2. Delete all documents in this folder
        const docs = await Document.find({ folder: folderId });
        for (const doc of docs) {
            // Delete file from disk
            const filePath = path.join(path.resolve(), doc.url.substring(1));
            if (fs.existsSync(filePath)) {
                try { fs.unlinkSync(filePath); } catch (_) { }
            }
            await doc.deleteOne();
        }

        // 3. Delete the folder itself
        await Folder.findByIdAndDelete(folderId);
    };
    
    // Notify Team before recursive deletion
    const io = req.app.get('socketio');
    const team = await Team.findById(folder.team); // Need team for member IDs
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
                link: '/document-share'
            }, io);
        }
    }

    await deleteFolderRecursive(folder._id);
    emitTeamUpdate(io, teamId, 'FOLDER_DELETE');

    res.json({ message: 'Folder deleted' });
});

// @desc    Download document
// @route   GET /api/documents/download/:id
// @access  Private
const downloadDocument = asyncHandler(async (req, res) => {
    const document = await Document.findById(req.params.id);

    if (!document) {
        res.status(404);
        throw new Error('Document not found');
    }

    // Verify the requester is a team member or owner OR Master Admin
    const team = await Team.findById(document.team);
    if (!team) {
        res.status(404);
        throw new Error('Team not found');
    }

    const userId = req.user.id.toString();
    const isMasterAdmin = req.user.role === 'master_admin';
    const isHead = isMasterAdmin || req.user.role === 'team_head';
    const isMember = isMasterAdmin || team.members.some(m => m && m.toString() === userId) || team.owner.toString() === userId;
    
    if (!isMember) {
        res.status(403);
        throw new Error('Not authorized to download this file');
    }

    // Enforce the per-document downloadable flag for regular members
    if (!document.isDownloadable && !isHead) {
        res.status(403);
        throw new Error('Downloads have been restricted for this file by the team head');
    }

    // Resolve the file path on disk
    const filePath = path.join(path.resolve(), document.url.replace(/^\//, ''));
    if (!fs.existsSync(filePath)) {
        res.status(404);
        throw new Error('File not found on server');
    }

    // Set proper headers for download to make it seamless
    res.setHeader('Content-Disposition', `attachment; filename="${document.name}"`);
    res.setHeader('Content-Type', 'application/octet-stream');
    res.setHeader('Access-Control-Expose-Headers', 'Content-Disposition');

    const fileStream = fs.createReadStream(filePath);
    fileStream.pipe(res);
});

// @desc    Delete document
// @route   DELETE /api/documents/:id
// @access  Private
const deleteDocument = asyncHandler(async (req, res) => {
    const document = await Document.findById(req.params.id);

    if (!document) {
        res.status(404);
        throw new Error('Document not found');
    }

    // Check permission: Uploader OR Team Head OR Master Admin can delete
    const isUploader = document.uploadedBy.toString() === req.user.id;
    const isMasterAdmin = req.user.role === 'master_admin';
    const isHead = isMasterAdmin || req.user.role === 'team_head' || req.user.role === 'admin';

    if (!isUploader && !isHead) {
        res.status(403);
        throw new Error('Not authorized to delete this file');
    }

    const teamId = document.team;

    // Remove file from filesystem
    try {
        const filePath = path.join(path.resolve(), document.url.replace(/^\//, '')); // remove leading /
        if (fs.existsSync(filePath)) {
            fs.unlinkSync(filePath);
        }
    } catch (err) {
        console.error("Error deleting file from disk:", err);
    }

    await document.deleteOne();

    // Notify Team
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
                link: '/document-share'
            }, io);
        }
    }
    
    emitTeamUpdate(io, teamId, 'DOCUMENT_DELETE');

    res.json({ id: req.params.id });
});

// @desc    Toggle isDownloadable flag for a document
// @route   PATCH /api/documents/:id/downloadable
// @access  Private (Uploader, Team Head, Master Admin)
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

export { getDocuments, uploadDocument, downloadDocument, deleteDocument, createFolder, renameFolder, deleteFolder, toggleDownloadable };
