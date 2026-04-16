import mongoose from 'mongoose';

const documentSchema = mongoose.Schema({
    user: {
        type: mongoose.Schema.Types.ObjectId,
        required: true, // Legacy field, kept for safety
        ref: 'User',
    },
    uploadedBy: {
        type: mongoose.Schema.Types.ObjectId,
        required: true,
        ref: 'User',
    },
    name: {
        type: String,
        required: true,
    },
    type: {
        type: String,
        required: true,
    },
    size: {
        type: String,
        required: true,
    },
    url: {
        type: String,
        required: true,
    },
    folder: {
        type: mongoose.Schema.Types.ObjectId,
        ref: 'Folder',
        default: null,
    },
    team: {
        type: mongoose.Schema.Types.ObjectId,
        ref: 'Team',
        required: true
    },
    isDownloadable: {
        type: Boolean,
        default: true   // All roles can download by default; head/admin can restrict
    },
}, {
    timestamps: true,
});

const Document = mongoose.model('Document', documentSchema);

export default Document;
