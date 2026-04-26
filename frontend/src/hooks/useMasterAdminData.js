import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useAuth } from '../context/AuthContext';
import { toast } from 'sonner';

export const useMasterAdminData = () => {
    const { user, impersonate } = useAuth();
    const queryClient = useQueryClient();

    const headers = {
        'Authorization': `Bearer ${user?.token}`,
        'Content-Type': 'application/json'
    };

    // ─── Queries ───────────────────────────────────────────────

    const fetchStats = async () => {
        const res = await fetch('/api/admin/stats', { headers });
        if (!res.ok) throw new Error('Failed to fetch stats');
        return res.json();
    };

    const fetchTeams = async () => {
        const res = await fetch('/api/admin/teams', { headers });
        if (!res.ok) throw new Error('Failed to fetch teams');
        return res.json();
    };

    const fetchUsers = async () => {
        const res = await fetch('/api/admin/users', { headers });
        if (!res.ok) throw new Error('Failed to fetch users');
        return res.json();
    };

    const fetchAuditLogs = async () => {
        const res = await fetch('/api/admin/audit-logs', { headers });
        if (!res.ok) throw new Error('Failed to fetch audit logs');
        return res.json();
    };

    const fetchPendingUsers = async () => {
        const res = await fetch('/api/admin/users/pending', { headers });
        if (!res.ok) throw new Error('Failed to fetch pending users');
        return res.json();
    };

    const { data: stats } = useQuery({ queryKey: ['adminStats'], queryFn: fetchStats, enabled: !!user });
    const { data: teams = [], isLoading: loadingTeams } = useQuery({ queryKey: ['adminTeams'], queryFn: fetchTeams, enabled: !!user });
    const { data: users = [], isLoading: loadingUsers } = useQuery({ queryKey: ['adminUsers'], queryFn: fetchUsers, enabled: !!user });
    const { data: auditLogs = [], isLoading: loadingLogs } = useQuery({ queryKey: ['adminAuditLogs'], queryFn: fetchAuditLogs, enabled: !!user });
    const { data: pendingUsers = [], isLoading: loadingPending } = useQuery({ queryKey: ['adminPendingUsers'], queryFn: fetchPendingUsers, enabled: !!user });

    // ─── Mutations ───────────────────────────────────────────────────

    const deleteTeamMutation = useMutation({
        mutationFn: async (teamId) => {
            const res = await fetch(`/api/admin/teams/${teamId}`, { method: 'DELETE', headers });
            if (!res.ok) throw new Error('Failed to delete team');
            return res.json();
        },
        onSuccess: () => {
            queryClient.invalidateQueries(['adminTeams']);
            queryClient.invalidateQueries(['adminStats']);
            toast.success('Team deleted globally');
        },
        onError: (err) => toast.error(err.message)
    });

    const deleteUserMutation = useMutation({
        mutationFn: async (userId) => {
            const res = await fetch(`/api/admin/users/${userId}`, { method: 'DELETE', headers });
            if (!res.ok) throw new Error(await res.text());
            return res.json();
        },
        onSuccess: () => {
            queryClient.invalidateQueries(['adminUsers']);
            queryClient.invalidateQueries(['adminTeams']);
            queryClient.invalidateQueries(['adminStats']);
            toast.success('User deleted completely');
        },
        onError: (err) => toast.error('Failed to delete user')
    });

    const updateUserMutation = useMutation({
        mutationFn: async ({ userId, name, email, password }) => {
            const res = await fetch(`/api/admin/users/${userId}`, {
                method: 'PUT',
                headers,
                body: JSON.stringify({ name, email, password })
            });
            if (!res.ok) {
                const errorData = await res.json();
                throw new Error(errorData.message || 'Failed to update user');
            }
            return res.json();
        },
        onSuccess: () => {
            queryClient.invalidateQueries(['adminUsers']);
            toast.success('User updated successfully');
        },
        onError: (err) => toast.error(err.message)
    });

    const addMemberMutation = useMutation({
        mutationFn: async ({ teamId, email }) => {
            const res = await fetch(`/api/admin/teams/${teamId}/members`, {
                method: 'POST',
                headers,
                body: JSON.stringify({ email })
            });
            if (!res.ok) {
                const errorData = await res.json();
                throw new Error(errorData.message || 'Failed to add member');
            }
            return res.json();
        },
        onSuccess: () => {
            queryClient.invalidateQueries(['adminTeams']);
            toast.success('User added to team');
        },
        onError: (err) => toast.error(err.message)
    });

    const removeMemberMutation = useMutation({
        mutationFn: async ({ teamId, memberId }) => {
            const res = await fetch(`/api/admin/teams/${teamId}/members/${memberId}`, {
                method: 'DELETE',
                headers
            });
            if (!res.ok) {
                const errorData = await res.json();
                throw new Error(errorData.message || 'Failed to remove member');
            }
            return res.json();
        },
        onSuccess: () => {
            queryClient.invalidateQueries(['adminTeams']);
            toast.success('User removed from team');
        },
        onError: (err) => toast.error(err.message)
    });

    const toggleSuspendMutation = useMutation({
        mutationFn: async (userId) => {
            const res = await fetch(`/api/admin/users/${userId}/suspend`, { method: 'PATCH', headers });
            if (!res.ok) throw new Error(await res.text());
            return res.json();
        },
        onSuccess: (data) => {
            queryClient.invalidateQueries(['adminUsers']);
            queryClient.invalidateQueries(['adminStats']);
            queryClient.invalidateQueries(['adminAuditLogs']);
            toast.success(data.status === 'active' ? 'User reactivated successfully' : 'User suspended successfully');
        },
        onError: (err) => toast.error('Failed to toggle suspension')
    });

    const approveUserMutation = useMutation({
        mutationFn: async (userId) => {
            const res = await fetch(`/api/admin/users/${userId}/approve`, { method: 'POST', headers });
            if (!res.ok) throw new Error(await res.text());
            return res.json();
        },
        onSuccess: (data) => {
            queryClient.invalidateQueries(['adminPendingUsers']);
            queryClient.invalidateQueries(['adminUsers']);
            queryClient.invalidateQueries(['adminStats']);
            queryClient.invalidateQueries(['adminAuditLogs']);
            toast.success(data.message);
        },
        onError: (err) => toast.error('Failed to approve user')
    });

    const rejectUserMutation = useMutation({
        mutationFn: async (userId) => {
            const res = await fetch(`/api/admin/users/${userId}/reject`, { method: 'DELETE', headers });
            if (!res.ok) throw new Error(await res.text());
            return res.json();
        },
        onSuccess: (data) => {
            queryClient.invalidateQueries(['adminPendingUsers']);
            queryClient.invalidateQueries(['adminStats']);
            queryClient.invalidateQueries(['adminAuditLogs']);
            toast.success(data.message);
        },
        onError: (err) => toast.error('Failed to reject user')
    });

    const transferTeamMutation = useMutation({
        mutationFn: async ({ teamId, newOwnerId }) => {
            const res = await fetch(`/api/admin/teams/${teamId}/transfer`, {
                method: 'PATCH', headers, body: JSON.stringify({ newOwnerId })
            });
            if (!res.ok) throw new Error(await res.text());
            return res.json();
        },
        onSuccess: () => {
            queryClient.invalidateQueries(['adminTeams']);
            queryClient.invalidateQueries(['adminAuditLogs']);
            toast.success('Team ownership transferred');
        },
        onError: (err) => toast.error('Transfer failed')
    });

    const updateTeamMutation = useMutation({
        mutationFn: async ({ teamId, name, description }) => {
            const res = await fetch(`/api/admin/teams/${teamId}`, {
                method: 'PUT', headers, body: JSON.stringify({ name, description })
            });
            if (!res.ok) throw new Error(await res.text());
            return res.json();
        },
        onSuccess: () => {
            queryClient.invalidateQueries(['adminTeams']);
            queryClient.invalidateQueries(['adminAuditLogs']);
            toast.success('Team details updated');
        },
        onError: (err) => toast.error('Update failed')
    });

    const impersonateMutation = useMutation({
        mutationFn: async (userId) => {
            return impersonate(userId);
        },
        onSuccess: (result) => {
            if (result.success) {
                toast.success('Entering Shadow Mode...');
                // Redirection will happen naturally as user state changes, 
                // but we might want to force a redirect to home.
                window.location.href = '/'; 
            } else {
                toast.error(result.error);
            }
        },
        onError: (err) => toast.error('Impersonation failed')
    });

    return {
        user,
        stats,
        teams,
        loadingTeams,
        users,
        loadingUsers,
        auditLogs,
        loadingLogs,
        pendingUsers,
        loadingPending,
        mutations: {
            deleteTeam: deleteTeamMutation,
            deleteUser: deleteUserMutation,
            updateUser: updateUserMutation,
            addMember: addMemberMutation,
            removeMember: removeMemberMutation,
            toggleSuspend: toggleSuspendMutation,
            approveUser: approveUserMutation,
            rejectUser: rejectUserMutation,
            transferTeam: transferTeamMutation,
            updateTeam: updateTeamMutation,
            impersonateUser: impersonateMutation
        }
    };
};
