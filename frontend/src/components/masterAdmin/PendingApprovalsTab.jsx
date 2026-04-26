import React from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Shield, CheckCircle } from 'lucide-react';

const ROLE_LABELS = {
    team_head: 'Team Head',
    admin: 'Admin',
};

const PendingApprovalsTab = ({ pendingUsers, loadingPending, mutations }) => {
    const handleApprove = (id, name) => {
        if (window.confirm(`Approve registration for ${name}?`)) {
            mutations.approveUser.mutate(id);
        }
    };

    const handleReject = (id, name) => {
        if (window.confirm(`Reject registration for ${name}? Their request will be completely deleted.`)) {
            mutations.rejectUser.mutate(id);
        }
    };

    const getInitials = (name) => {
        if (!name) return 'U';
        const parts = name.trim().split(' ');
        if (parts.length === 1) return parts[0].charAt(0).toUpperCase();
        return (parts[0].charAt(0) + parts[parts.length - 1].charAt(0)).toUpperCase();
    };

    if (loadingPending) return <div className="text-center py-10 text-gray-500 font-medium">Checking for pending requests...</div>;

    if (pendingUsers.length === 0) {
        return (
            <div className="bg-white/70 dark:bg-gray-800/70 backdrop-blur-xl border border-gray-200/50 dark:border-gray-700/50 rounded-2xl shadow-xl overflow-hidden p-6 max-w-4xl mx-auto">
                <div className="text-center py-12 px-4 rounded-2xl border-2 border-dashed border-gray-200 dark:border-gray-700 bg-gray-50/50 dark:bg-gray-900/30">
                    <div className="w-16 h-16 bg-gray-100 dark:bg-gray-800 rounded-full flex items-center justify-center mx-auto mb-4 text-gray-400">
                        <CheckCircle size={32} />
                    </div>
                    <h3 className="text-lg font-bold text-gray-900 dark:text-white mb-1">All caught up!</h3>
                    <p className="text-sm text-gray-500">There are no pending registrations requiring your approval right now.</p>
                </div>
            </div>
        );
    }

    return (
        <div className="bg-white/70 dark:bg-gray-800/70 backdrop-blur-xl border border-gray-200/50 dark:border-gray-700/50 rounded-2xl shadow-xl overflow-hidden p-6 max-w-4xl mx-auto">
            <h2 className="text-xl font-bold text-gray-900 dark:text-white mb-6 flex items-center gap-2">
                <Shield className="text-amber-500" size={24} />
                Registration Approvals
            </h2>
            
            <div className="space-y-4">
                <AnimatePresence>
                    {pendingUsers.map((u) => (
                        <motion.div
                            key={u._id}
                            initial={{ opacity: 0, scale: 0.98 }}
                            animate={{ opacity: 1, scale: 1 }}
                            exit={{ opacity: 0, scale: 0.95 }}
                            className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-5 rounded-2xl bg-white dark:bg-gray-800 border border-gray-200/50 dark:border-gray-700 shadow-sm hover:shadow-md transition-all"
                        >
                            <div className="flex items-center gap-4">
                                <div className="w-14 h-14 rounded-2xl bg-amber-50 dark:bg-amber-900/20 text-amber-600 dark:text-amber-400 flex items-center justify-center font-black text-xl border border-amber-100 dark:border-amber-900/30 shadow-inner shrink-0">
                                    {getInitials(u.name)}
                                </div>
                                <div>
                                    <h3 className="text-lg font-bold text-gray-900 dark:text-white">{u.name}</h3>
                                    <p className="text-sm text-gray-500 mb-1">{u.email}</p>
                                    <div className="flex items-center gap-2">
                                        <span className="px-2.5 py-0.5 rounded-full text-xs font-bold bg-indigo-100 dark:bg-indigo-900/40 text-indigo-700 dark:text-indigo-400 inline-block">
                                            Requested Role: {ROLE_LABELS[u.role] ?? u.role}
                                        </span>
                                        <span className="text-xs text-gray-400">
                                            {new Date(u.createdAt).toLocaleDateString()}
                                        </span>
                                    </div>
                                </div>
                            </div>
                            
                            <div className="flex items-center gap-3 w-full sm:w-auto">
                                <button
                                    onClick={() => handleReject(u._id, u.name)}
                                    disabled={mutations.rejectUser.isLoading}
                                    className="flex-1 sm:flex-none px-4 py-2 border border-red-200 dark:border-red-900/50 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-xl text-sm font-bold transition-colors shadow-sm disabled:opacity-50"
                                >
                                    Reject &amp; Delete
                                </button>
                                <button
                                    onClick={() => handleApprove(u._id, u.name)}
                                    disabled={mutations.approveUser.isLoading}
                                    className="flex-1 sm:flex-none px-6 py-2 bg-gradient-to-r from-emerald-500 to-teal-500 hover:from-emerald-600 hover:to-teal-600 text-white rounded-xl text-sm font-bold shadow-md shadow-emerald-500/20 transition-all disabled:opacity-50"
                                >
                                    Approve
                                </button>
                            </div>
                        </motion.div>
                    ))}
                </AnimatePresence>
            </div>
        </div>
    );
};

export default PendingApprovalsTab;

