import express from 'express';
import { getDocuments, uploadDocument, downloadDocument, deleteDocument, createFolder, renameFolder, deleteFolder, toggleDownloadable } from '../controllers/documentController.js';
import { protect } from '../middleware/authMiddleware.js';

const router = express.Router();

router.use(protect);

router.route('/')
    .get(getDocuments);

router.post('/upload', uploadDocument);
router.get('/download/:id', downloadDocument);

router.post('/folder', createFolder);
router.route('/folder/:id')
    .put(renameFolder)
    .delete(deleteFolder);

router.route('/:id')
    .delete(deleteDocument);

router.patch('/:id/downloadable', toggleDownloadable);

export default router;
