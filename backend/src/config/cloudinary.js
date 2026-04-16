import { v2 as cloudinary } from 'cloudinary';
import { CloudinaryStorage } from 'multer-storage-cloudinary';
import multer from 'multer';
import path from 'path';
import fs from 'fs';

// ---------- Determine if Cloudinary is configured ----------
const CLOUDINARY_ENABLED =
    !!process.env.CLOUDINARY_CLOUD_NAME &&
    process.env.CLOUDINARY_CLOUD_NAME !== 'your_cloud_name' &&
    !!process.env.CLOUDINARY_API_KEY &&
    !!process.env.CLOUDINARY_API_SECRET;

if (CLOUDINARY_ENABLED) {
    cloudinary.config({
        cloud_name: process.env.CLOUDINARY_CLOUD_NAME,
        api_key:    process.env.CLOUDINARY_API_KEY,
        api_secret: process.env.CLOUDINARY_API_SECRET,
    });
    console.log('[Storage] Cloudinary configured — uploads will go to the cloud.');
} else {
    console.log('[Storage] Cloudinary not configured — uploads will be stored locally in /uploads.');
}

// ---------- Multer storage — Cloudinary or local disk ----------
const diskStorage = multer.diskStorage({
    destination(req, file, cb) {
        const uploadsDir = path.join(path.resolve(), 'uploads');
        if (!fs.existsSync(uploadsDir)) fs.mkdirSync(uploadsDir, { recursive: true });
        cb(null, uploadsDir);
    },
    filename(req, file, cb) {
        cb(null, `${Date.now()}-${file.originalname.replace(/[^a-zA-Z0-9._-]/g, '_')}`);
    },
});

const cloudinaryStorage = CLOUDINARY_ENABLED
    ? new CloudinaryStorage({
        cloudinary,
        params: async (req, file) => ({
            folder: 'aurora-docs',
            resource_type: 'auto',
            // Keep original filename as public_id (sanitized)
            public_id: `${Date.now()}-${file.originalname
                .replace(/\.[^/.]+$/, '')
                .replace(/[^a-zA-Z0-9_-]/g, '_')}`,
            // Don't convert file formats
            format: '',
        }),
    })
    : null;

export const upload = multer({
    storage: CLOUDINARY_ENABLED ? cloudinaryStorage : diskStorage,
    limits: { fileSize: 50 * 1024 * 1024 }, // 50 MB
}).single('file');

export { cloudinary, CLOUDINARY_ENABLED };
