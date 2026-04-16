import React, { createContext, useContext, useState, useEffect, useRef, useCallback } from 'react';
import { useAuth } from './AuthContext';
import io from 'socket.io-client';
import { logger } from '../utils/logger';
import { toast } from 'sonner';

const ringtoneSound = '/sounds/ringtone.mp3';

const ChatContext = createContext();

const SOCKET_URL = import.meta.env.PROD 
    ? 'https://digital-workspace.onrender.com' 
    : 'http://localhost:4000';


export const ChatProvider = ({ children }) => {
    const { user } = useAuth();
    const [activeChat, setActiveChat] = useState(null);
    const [chatsData, setChatsData] = useState({});
    const [socketConnected, setSocketConnected] = useState(false);
    const socketRef = useRef();
    const [totalUnreadCount, setTotalUnreadCount] = useState(0);
    const [isLoadingHistory, setIsLoadingHistory] = useState(false);
    const [fetchedChats, setFetchedChats] = useState(new Set());
    const sentMessageIdsRef = useRef(new Set());
    const processedMessageIdsRef = useRef(new Set());
    const [onlineUsers, setOnlineUsers] = useState(new Set());
    const [dbUnreadCount, setDbUnreadCount] = useState(0);

    const ringtoneRef = useRef(null);

    useEffect(() => {
        ringtoneRef.current = new Audio(`${ringtoneSound}?t=${Date.now()}`);
        ringtoneRef.current.loop = true;
    }, []);

    useEffect(() => {
        const count = Object.values(chatsData).reduce((total, chat) => {
            return total + (chat.unreadCount || 0);
        }, 0);
        setTotalUnreadCount(count);
    }, [chatsData]);

    const activeChatRef = useRef(activeChat);
    useEffect(() => {
        activeChatRef.current = activeChat;

        if (activeChat) {
            setChatsData(prev => {
                if (!prev[activeChat]) return prev;
                return {
                    ...prev,
                    [activeChat]: {
                        ...prev[activeChat],
                        unread: false,
                        unreadCount: 0
                    }
                };
            });
        }
    }, [activeChat]);

    const fetchUserChats = useCallback(async () => {
        const token = user?.token || JSON.parse(localStorage.getItem('user'))?.token;
        if (!token) return;

        try {
            const res = await fetch('/api/chat', {
                headers: { 'Authorization': `Bearer ${token}` }
            });
            const data = await res.json();

            if (res.ok) {
                setChatsData(prevChatsData => {
                    const newChatsData = { ...prevChatsData };
                    let hasChanges = false;

                    data.forEach(chat => {
                        let displayName = chat.chatName;
                        if (!chat.isGroupChat && chat.users) {
                            const otherUser = chat.users.find(u => u._id !== (user?._id || user?.id)) || chat.users[0];
                            displayName = otherUser?.name || "Unknown User";
                        }

                        let initialMessages = [];
                        if (chat.latestMessage) {
                            const msg = chat.latestMessage;
                            const senderName = msg.sender?.name || "Unknown";

                            initialMessages = [{
                                id: msg._id,
                                text: msg.text,
                                sender: senderName,
                                time: new Date(msg.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
                                isMe: (msg.sender?._id || msg.sender) === (user?._id || user?.id),
                                senderAvatar: senderName ? senderName[0] : 'U',
                                type: msg.type || 'text',
                                fullTime: msg.createdAt
                            }];
                        }

                        if (!newChatsData[chat._id]) {
                            newChatsData[chat._id] = {
                                id: chat._id,
                                name: displayName,
                                type: chat.isGroupChat ? 'group' : 'private',
                                isGroupChat: chat.isGroupChat,
                                users: chat.users || [],
                                groupAdmin: chat.groupAdmin,
                                onlineCount: chat.users?.length || 0,
                                messages: initialMessages,
                                unread: false,
                                unreadCount: 0
                            };
                            hasChanges = true;
                        }
                    });

                    return hasChanges ? newChatsData : prevChatsData;
                });
            }
        } catch (err) {
            logger.error("Failed to fetch chats", err);
        }
    }, [user]);

    useEffect(() => {
        const unlockAudio = () => {
            const audio = new Audio("data:audio/wav;base64,UklGRigAAABXQVZFZm10IBIAAAABAAEARKwAAIhYAQACABAAAABkYXRhAgAAAAEA");
            audio.play().catch(() => { });
            document.removeEventListener('click', unlockAudio);
            document.removeEventListener('keydown', unlockAudio);
        };

        document.addEventListener('click', unlockAudio);
        document.addEventListener('keydown', unlockAudio);

        return () => {
            document.removeEventListener('click', unlockAudio);
            document.removeEventListener('keydown', unlockAudio);
        };
    }, []);

    useEffect(() => {
        if (user) {
            fetchUserChats();
        }
    }, [user, fetchUserChats]);

    useEffect(() => {
        if (!user) return;

        socketRef.current = io(SOCKET_URL, {
            transports: ['websocket'],
            reconnection: true,
            reconnectionDelay: 1000,
            reconnectionAttempts: 10,
            timeout: 20000,
        });


        socketRef.current.on('connect', () => {
            logger.log('Socket connected/reconnected');
            socketRef.current.emit('setup', user);
            setSocketConnected(true);
            if (activeChatRef.current) {
                socketRef.current.emit('joinChat', activeChatRef.current);
            }
        });

        socketRef.current.on('disconnect', () => {
            logger.log('Socket disconnected');
            setSocketConnected(false);
            setOnlineUsers(new Set());
        });


        socketRef.current.on('onlineUsers', (users) => {
            setOnlineUsers(new Set(users));
        });

        socketRef.current.on('receiveMessage', (newMessage) => {
            const msgId = newMessage._id?.toString();
            if (msgId && processedMessageIdsRef.current.has(msgId)) return;
            if (msgId) processedMessageIdsRef.current.add(msgId);

            const senderId = newMessage.sender?._id?.toString() || newMessage.sender?.toString();
            const isOwnMessage = senderId && senderId === (user?._id?.toString() || user?.id?.toString());
            if (isOwnMessage) return;

            const formattedMsg = {
                id: newMessage._id,
                text: newMessage.text,
                sender: newMessage.sender.name,
                time: new Date(newMessage.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
                isMe: false,
                senderAvatar: newMessage.sender.name ? newMessage.sender.name[0] : 'U',
                type: newMessage.type || 'text',
                fullTime: newMessage.createdAt
            };

            let chatId = newMessage.chat?._id || newMessage.room;

            setChatsData(prev => {
                const isCurrentChat = chatId === activeChatRef.current;
                const existingChat = prev[chatId];

                if (existingChat) {
                    return {
                        ...prev,
                        [chatId]: {
                            ...existingChat,
                            messages: [...(existingChat.messages || []), formattedMsg],
                            unread: !isCurrentChat,
                            unreadCount: isCurrentChat ? 0 : (existingChat.unreadCount || 0) + 1
                        }
                    };
                }

                if (newMessage.chat) {
                    let displayName = newMessage.chat.chatName;
                    if (!newMessage.chat.isGroupChat && newMessage.chat.users) {
                        const otherUser = newMessage.chat.users.find(u => u._id !== user._id) || newMessage.chat.users[0];
                        displayName = otherUser?.name || "Unknown User";
                    }

                    return {
                        ...prev,
                        [chatId]: {
                            id: chatId,
                            name: displayName,
                            type: newMessage.chat.isGroupChat ? 'group' : 'private',
                            isGroupChat: newMessage.chat.isGroupChat,
                            users: newMessage.chat.users || [],
                            groupAdmin: newMessage.chat.groupAdmin,
                            messages: [formattedMsg],
                            unread: !isCurrentChat,
                            unreadCount: isCurrentChat ? 0 : 1
                        }
                    };
                }

                return prev;
            });

            if (chatId !== activeChatRef.current) {
            }
        });

        socketRef.current.on('callUser', ({ from, name, signal, isVideo }) => {
            logger.log('Receiving call from', name, 'Video:', isVideo);
            setCall({ isReceivingCall: true, from, name, signal, isVideo, iceCandidates: [] });
        });

        socketRef.current.on('ice-candidate', (candidate) => {
            setCall(prev => {
                if (!prev.isReceivingCall && !prev.userToCall) return prev;
                return {
                    ...prev,
                    iceCandidates: [...(prev.iceCandidates || []), candidate]
                };
            });
        });

        // Global listener: incoming DB notifications for ANY page
        // Keeps master admin bell badge live without needing /notifications page open
        socketRef.current.on('newNotification', (notification) => {
            setDbUnreadCount(prev => prev + 1);
            toast(notification.title, {
                description: notification.description,
                duration: 5000,
            });
        });

        return () => {
            socketRef.current?.disconnect();
        };
    }, [user]);

    const [call, setCall] = useState({});
    const [callAccepted, setCallAccepted] = useState(false);
    const [callEnded, setCallEnded] = useState(false);
    const [isCalling, setIsCalling] = useState(false);

    const startCall = (userId, userName, isVideo = true) => {
        setCallAccepted(false);
        setCallEnded(false);
        setIsCalling(true);
        setCall({ isReceivingCall: false, userToCall: userId, name: userName, isVideo, iceCandidates: [] });
    };

    const answerCall = () => {
        setCallAccepted(true);
    };

    const endCall = () => {
        setCall({});
        setCallAccepted(false);
        setCallEnded(true);
        setIsCalling(false);
        setTimeout(() => setCallEnded(false), 500);
    };



    useEffect(() => {
        if (call.isReceivingCall && !callAccepted) {
            logger.log("Attempting to play ringtone");
            ringtoneRef.current?.play()
                .then(() => logger.log("Ringtone played successfully"))
                .catch(e => logger.error("Ringtone play failed:", e));
        } else {
            if (ringtoneRef.current) {
                ringtoneRef.current.pause();
                ringtoneRef.current.currentTime = 0;
            }
        }
    }, [call.isReceivingCall, callAccepted]);

    const value = {
        activeChat,
        setActiveChat,
        chatsData,
        setChatsData,
        totalUnreadCount,
        dbUnreadCount,
        setDbUnreadCount,
        socketRef,
        socketConnected,
        isLoadingHistory,
        setIsLoadingHistory,
        fetchedChats,
        setFetchedChats,
        fetchUserChats,
        user,
        onlineUsers,
        chats: Object.values(chatsData),
        call,
        callAccepted,
        callEnded,
        isCalling,
        startCall,
        answerCall,
        endCall,
        setCallAccepted
    };

    return (
        <ChatContext.Provider value={value}>
            {children}
        </ChatContext.Provider>
    );
};

export const useChatContext = () => {
    const context = useContext(ChatContext);
    if (!context) {
        throw new Error('useChatContext must be used within a ChatProvider');
    }
    return context;
};
