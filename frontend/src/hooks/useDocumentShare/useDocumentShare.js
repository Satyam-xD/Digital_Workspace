import { useState, useEffect, useCallback } from 'react';
import { useAuth } from '../../context/AuthContext';
import { toast } from 'sonner';
import { FileText, PenTool, FileSpreadsheet, FileImage } from 'lucide-react';


export const useDocumentShare = () => {
    const { user } = useAuth();
    const [documents, setDocuments] = useState([]);
    const [folders, setFolders] = useState([]);
    const [loading, setLoading] = useState(true);
    const [searchTerm, setSearchTerm] = useState('');

    // Team State
    const [teams, setTeams] = useState([]);
    const [selectedTeam, setSelectedTeam] = useState(null);

    // Folder Navigation State
    const [currentFolder, setCurrentFolder] = useState(null); // null = root

    // Fetch Teams
    useEffect(() => {
        const fetchTeams = async () => {
            if (!user?.token) return;
            try {
                const res = await fetch('/api/team', {
                    headers: { 'Authorization': `Bearer ${user.token}` }
                });
                if (res.ok) {
                    const data = await res.json();
                    setTeams(data);
                    if (data.length > 0 && !selectedTeam) {
                        setSelectedTeam(data[0]);
                    }
                }
            } catch (err) {
                console.error("Error fetching teams", err);
            }
        };
        fetchTeams();
    }, [user]);

    const fetchDocuments = useCallback(async () => {
        if (!user || !user.token || !selectedTeam) return;
        setLoading(true);
        try {
            const folderQuery = currentFolder ? `&folderId=${currentFolder._id}` : `&folderId=null`;
            const response = await fetch(`/api/documents?teamId=${selectedTeam.id}${folderQuery}`, {
                headers: {
                    'Authorization': `Bearer ${user.token}`,
                },
            });
            const data = await response.json();
            if (response.ok) {
                setDocuments(data.documents);
                setFolders(data.folders);
            }
        } catch (error) {
            console.error('Error fetching documents:', error);
            toast.error("Failed to load documents");
        } finally {
            setLoading(false);
        }
    }, [user, selectedTeam, currentFolder]);

    useEffect(() => {
        fetchDocuments();
    }, [fetchDocuments]);

    const uploadFile = async (file) => {
        if (!file || !selectedTeam) return;

        const formData = new FormData();
        formData.append('file', file);
        formData.append('teamId', selectedTeam.id);
        if (currentFolder) {
            formData.append('folderId', currentFolder._id);
        } else {
            formData.append('folderId', 'null');
        }

        try {
            const response = await fetch('/api/documents/upload', {
                method: 'POST',
                headers: {
                    'Authorization': `Bearer ${user.token}`,
                    // Note: Content-Type is set automatically by browser with boundary for FormData
                },
                body: formData,
            });

            if (response.ok) {
                fetchDocuments();
                toast.success('File uploaded successfully');
            } else {
                toast.error('Failed to upload file');
            }
        } catch (error) {
            console.error('Error uploading:', error);
            toast.error('Error uploading file');
        }
    };

    const createFolder = async (name) => {
        if (!selectedTeam || !name) return;
        try {
            const response = await fetch('/api/documents/folder', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${user.token}`,
                },
                body: JSON.stringify({
                    name,
                    teamId: selectedTeam.id,
                    parentFolderId: currentFolder ? currentFolder._id : null
                })
            });

            if (response.ok) {
                fetchDocuments();
                toast.success('Folder created');
            } else {
                toast.error('Failed to create folder');
            }
        } catch (error) {
            console.error("Create folder error", error);
            toast.error('Failed to create folder');
        }
    };

    const renameFolder = async (folderId, newName) => {
        try {
            const response = await fetch(`/api/documents/folder/${folderId}`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${user.token}`,
                },
                body: JSON.stringify({ name: newName })
            });
            if (response.ok) {
                fetchDocuments();
                toast.success("Folder renamed");
            } else {
                toast.error("Failed to rename (Admin only)");
            }
        } catch (error) {
            toast.error("Error renaming folder");
        }
    }

    const deleteFolder = async (folderId) => {
        if (!window.confirm("Delete this folder and all contents?")) return;
        try {
            const response = await fetch(`/api/documents/folder/${folderId}`, {
                method: 'DELETE',
                headers: {
                    'Authorization': `Bearer ${user.token}`,
                }
            });
            if (response.ok) {
                fetchDocuments();
                toast.success("Folder deleted");
            } else {
                toast.error("Failed to delete (Admin only)");
            }
        } catch (error) {
            toast.error("Error deleting folder");
        }
    }

    const handleDownload = async (doc) => {
        try {
            const response = await fetch(`/api/documents/download/${doc._id}`, {
                headers: {
                    'Authorization': `Bearer ${user.token}`,
                },
            });

            if (!response.ok) {
                toast.error('Failed to download file');
                return;
            }

            const blob = await response.blob();
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = doc.name;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            window.URL.revokeObjectURL(url);
            toast.success('Download started');
        } catch (error) {
            console.error('Download error:', error);
            toast.error('Failed to download file');
        }
    };

    const handleShare = (doc) => {
        // Since URL is relative, construct full URL
        const link = `${window.location.origin}${doc.url}`;
        navigator.clipboard.writeText(link);
        toast.success('Link copied to clipboard');
    };

    const handleDelete = async (id) => {
        if (!window.confirm("Delete this file?")) return;

        try {
            const response = await fetch(`/api/documents/${id}`, {
                method: 'DELETE',
                headers: {
                    'Authorization': `Bearer ${user.token}`,
                }
            });
            if (response.ok) {
                setDocuments(documents.filter(d => d._id !== id));
                toast.success('File deleted');
            } else {
                toast.error("Failed to delete (Permission denied)");
            }
        } catch (error) {
            console.error("Delete failed", error);
            toast.error("Delete failed");
        }
    };

    const toggleDownloadable = async (doc) => {
        try {
            const response = await fetch(`/api/documents/${doc._id}/downloadable`, {
                method: 'PATCH',
                headers: { 'Authorization': `Bearer ${user.token}` }
            });
            if (response.ok) {
                const updated = await response.json();
                setDocuments(prev =>
                    prev.map(d => d._id === doc._id
                        ? { ...d, isDownloadable: updated.isDownloadable }
                        : d
                    )
                );
                toast.success(`Download ${updated.isDownloadable ? 'enabled' : 'restricted'} for "${doc.name}"`);
            } else {
                const err = await response.json();
                toast.error(err.message || 'Failed to update download permission');
            }
        } catch (error) {
            console.error('toggleDownloadable error:', error);
            toast.error('Failed to update download permission');
        }
    };

    const getFileIcon = (type) => {
        if (!type) return { icon: FileText, color: 'text-gray-500 bg-gray-100 dark:bg-gray-700/50' };
        switch (type.toLowerCase()) {
            case 'pdf': return { icon: FileText, color: 'text-red-500 bg-red-100 dark:bg-red-900/20' };
            case 'figma': case 'fig': return { icon: PenTool, color: 'text-purple-500 bg-purple-100 dark:bg-purple-900/20' };
            case 'xls': case 'xlsx': return { icon: FileSpreadsheet, color: 'text-green-500 bg-green-100 dark:bg-green-900/20' };
            case 'doc': case 'docx': return { icon: FileText, color: 'text-blue-500 bg-blue-100 dark:bg-blue-900/20' };
            case 'png': case 'jpg': case 'jpeg': return { icon: FileImage, color: 'text-yellow-500 bg-yellow-100 dark:bg-yellow-900/20' };
            default: return { icon: FileText, color: 'text-gray-500 bg-gray-100 dark:bg-gray-700/50' };
        }
    };

    const filteredDocs = (documents || []).filter(doc =>
        doc?.name?.toLowerCase().includes(searchTerm.toLowerCase())
    );

    const filteredFolders = (folders || []).filter(f =>
        f?.name?.toLowerCase().includes(searchTerm.toLowerCase())
    );

    return {
        documents,
        folders,
        currentFolder,
        setCurrentFolder,
        teams,
        selectedTeam,
        setSelectedTeam,

        loading,
        searchTerm,
        setSearchTerm,
        fetchDocuments, // Exported for manual refresh
        uploadFile,
        createFolder,
        renameFolder,
        deleteFolder,
        handleDelete,
        handleDownload,
        handleShare,
        toggleDownloadable,
        getFileIcon,
        filteredDocs,
        filteredFolders
    };
};
