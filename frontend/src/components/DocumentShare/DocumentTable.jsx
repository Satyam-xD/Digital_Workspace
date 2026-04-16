
import React from 'react';
import { Download, Share2, Trash2, Lock, Unlock } from 'lucide-react';

const canManage = (role) =>
    role === 'master_admin' || role === 'team_head';

const DocumentTable = ({
    loading,
    filteredDocs,
    handleDelete,
    handleDownload,
    handleShare,
    getFileIcon,
    toggleDownloadable,
    userRole
}) => {
    const isManager = canManage(userRole);

    return (
        <div className="bg-white dark:bg-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 shadow-sm overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
                <h3 className="font-bold text-gray-900 dark:text-white">Recent Files</h3>
                <span className="text-sm text-gray-500">{filteredDocs.length} files</span>
            </div>
            <div className="overflow-x-auto">
                <table className="w-full">
                    <thead className="bg-gray-50 dark:bg-gray-700/50">
                        <tr>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Name</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Owner</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Date</th>
                            <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Size</th>
                            <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-200 dark:divide-gray-700">
                        {loading ? (
                            <tr><td colSpan="5" className="p-8 text-center text-gray-500">Loading documents...</td></tr>
                        ) : filteredDocs.length === 0 ? (
                            <tr><td colSpan="5" className="p-8 text-center text-gray-500">No documents found. Upload one!</td></tr>
                        ) : (
                            filteredDocs.map((doc) => {
                                const { icon: FileIcon, color } = getFileIcon(doc.type);
                                const downloadable = doc.isDownloadable !== false; // default true for old docs

                                return (
                                    <tr key={doc._id} className="hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors">
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <div className="flex items-center gap-3">
                                                <div className={`flex-shrink-0 h-10 w-10 rounded-lg flex items-center justify-center ${color}`}>
                                                    <FileIcon size={20} />
                                                </div>
                                                <div>
                                                    <div className="text-sm font-medium text-gray-900 dark:text-white truncate max-w-[200px]" title={doc.name}>
                                                        {doc.name}
                                                    </div>
                                                    <div className="flex items-center gap-1.5 mt-0.5">
                                                        <span className="text-xs text-gray-500">{(doc.type || 'unknown').toUpperCase()} File</span>
                                                        {/* Per-doc download badge */}
                                                        {!downloadable && (
                                                            <span className="inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded-full bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-400 text-[10px] font-semibold">
                                                                <Lock size={9} /> Restricted
                                                            </span>
                                                        )}
                                                    </div>
                                                </div>
                                            </div>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <div className="flex items-center">
                                                <div className="h-6 w-6 rounded-full bg-gradient-to-br from-aurora-500 to-purple-500 flex items-center justify-center text-[10px] text-white font-bold">
                                                    {doc.uploadedBy?.name ? doc.uploadedBy.name.charAt(0).toUpperCase() : 'U'}
                                                </div>
                                                <span className="ml-2 text-sm text-gray-600 dark:text-gray-300">
                                                    {doc.uploadedBy?.name || 'Unknown'}
                                                </span>
                                            </div>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                                            {new Date(doc.createdAt).toLocaleDateString()}
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                                            {doc.size}
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                                            <div className="flex items-center justify-end space-x-1">

                                                {/* Download button — always shown to managers; shown to members only when allowed */}
                                                {(isManager || downloadable) ? (
                                                    <button
                                                        onClick={() => handleDownload(doc)}
                                                        className="p-2 text-gray-400 hover:text-indigo-600 dark:hover:text-indigo-400 rounded-lg hover:bg-indigo-50 dark:hover:bg-indigo-900/20 transition-colors"
                                                        title="Download"
                                                    >
                                                        <Download size={16} />
                                                    </button>
                                                ) : (
                                                    <span
                                                        className="p-2 text-amber-400 cursor-not-allowed rounded-lg"
                                                        title="Downloads restricted by team head"
                                                    >
                                                        <Lock size={16} />
                                                    </span>
                                                )}

                                                {/* Share (copy link) */}
                                                <button
                                                    onClick={() => handleShare(doc)}
                                                    className="p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                                                    title="Copy link"
                                                >
                                                    <Share2 size={16} />
                                                </button>

                                                {/* Download-lock toggle — managers only */}
                                                {isManager && (
                                                    <button
                                                        onClick={() => toggleDownloadable(doc)}
                                                        className={`p-2 rounded-lg transition-colors ${downloadable
                                                            ? 'text-emerald-500 hover:text-amber-500 hover:bg-amber-50 dark:hover:bg-amber-900/20'
                                                            : 'text-amber-500 hover:text-emerald-500 hover:bg-emerald-50 dark:hover:bg-emerald-900/20'
                                                        }`}
                                                        title={downloadable ? 'Restrict downloads for members' : 'Allow downloads for all'}
                                                    >
                                                        {downloadable ? <Unlock size={16} /> : <Lock size={16} />}
                                                    </button>
                                                )}

                                                {/* Delete — manager only */}
                                                {isManager && (
                                                    <button
                                                        onClick={() => handleDelete(doc._id)}
                                                        className="p-2 text-gray-400 hover:text-red-500 dark:hover:text-red-400 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                                                        title="Delete"
                                                    >
                                                        <Trash2 size={16} />
                                                    </button>
                                                )}
                                            </div>
                                        </td>
                                    </tr>
                                );
                            })
                        )}
                    </tbody>
                </table>
            </div>
        </div>
    );
};

export default DocumentTable;
