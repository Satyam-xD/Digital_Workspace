import React, { useState, useEffect, useRef } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { Video, MessageCircle, FileText, Home, Bell, Menu, X, LogOut, Lock, Sun, Moon, Kanban, LogIn, Users, Shield, ChevronDown, Settings, Calendar } from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import { useTheme } from '../context/ThemeContext';
import { useChatContext } from '../context/ChatContext';
import { motion, AnimatePresence } from 'framer-motion';

const getInitials = (name) => {
  if (!name) return 'U';
  const parts = name.trim().split(' ');
  if (parts.length === 1) return parts[0].charAt(0).toUpperCase();
  return (parts[0].charAt(0) + parts[parts.length - 1].charAt(0)).toUpperCase();
};

const Header = () => {
  const location = useLocation();
  const navigate = useNavigate();
  const { user, logout } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const { totalUnreadCount, dbUnreadCount } = useChatContext();
  const unreadCount = totalUnreadCount + dbUnreadCount;
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [showUserMenu, setShowUserMenu] = useState(false);
  const userMenuRef = useRef(null);
  const [scrolled, setScrolled] = useState(false);

  // Handle scroll effect
  useEffect(() => {
    const handleScroll = () => {
      setScrolled(window.scrollY > 10);
    };
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  // Close user menu when clicking outside
  useEffect(() => {
    const handleClickOutside = (event) => {
      if (userMenuRef.current && !userMenuRef.current.contains(event.target)) {
        setShowUserMenu(false);
      }
    };

    if (showUserMenu) {
      document.addEventListener('mousedown', handleClickOutside);
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [showUserMenu]);

  // Close mobile menu on route change
  useEffect(() => {
    setIsMobileMenuOpen(false);
  }, [location.pathname]);

  const currentNavItems = React.useMemo(() => {
    if (!user) {
      return [
        { path: '/', icon: Home, label: 'Home' },
        { path: '/login', icon: LogIn, label: 'Login' }
      ];
    }
    const items = [
      { path: '/', icon: Home, label: 'Home' },
      { path: '/video-call', icon: Video, label: 'Meet' },
      { path: '/chat', icon: MessageCircle, label: 'Chat' },
      { path: '/document-share', icon: FileText, label: 'Docs' },
      { path: '/password-manager', icon: Lock, label: 'Vault' },
      { path: '/kanban', icon: Kanban, label: 'Board' },
      { path: '/calendar', icon: Calendar, label: 'Calendar' },
      { path: '/team-management', icon: Users, label: 'Team' }
    ];
    if (user.role === 'master_admin') {
      items.push({ path: '/admin', icon: Shield, label: 'Admin' });
    }
    return items;
  }, [user]);

  const handleLogout = () => {
    logout();
    navigate('/login');
    setShowUserMenu(false);
  };

  const initials = React.useMemo(() => getInitials(user?.name), [user?.name]);
  const { adminUser } = useAuth();

  return (
    <header
      style={{ top: adminUser ? '48px' : '0' }}
      className={`fixed left-0 right-0 z-50 transition-all duration-300 ${scrolled
        ? 'bg-white/90 dark:bg-gray-900/90 backdrop-blur-lg shadow-sm border-b border-gray-200/50 dark:border-gray-800/50 py-3'
        : 'bg-transparent py-5'
        }`}
    >
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex justify-between items-center">
          {/* Logo */}
          <Link to="/" className="flex items-center space-x-1 group relative z-50">
            <div className="relative w-12 h-12 flex items-center justify-center">
              <img src="/aurora-logo.svg" alt="Aurora Logo" className="relative w-10 h-10 object-contain transform group-hover:scale-110 transition-transform duration-300 dropdown-slide animate-color-glow" />
            </div>
            <div className="flex flex-col">
              <span className="text-xl font-bold bg-clip-text text-transparent bg-gradient-to-r from-gray-900 via-indigo-800 to-gray-900 dark:from-white dark:via-indigo-200 dark:to-white tracking-tight leading-none">
                Aurora
              </span>
            </div>
          </Link>

          {/* Desktop Navigation */}
          <nav className="hidden lg:flex items-center space-x-1 bg-white/50 dark:bg-gray-800/50 backdrop-blur-md px-2 py-1.5 rounded-full border border-gray-200/50 dark:border-gray-700/50 shadow-sm">
            {currentNavItems.map((item) => {
              const Icon = item.icon;
              const isActive = location.pathname === item.path;

              return (
                <Link
                  key={item.path}
                  to={item.path}
                  className="relative px-4 py-2 rounded-full text-sm font-medium transition-all duration-200 group"
                >
                  <div className="relative flex items-center space-x-2 z-10">
                    <Icon size={16} className={isActive ? 'text-indigo-600 dark:text-indigo-400 drop-shadow-[0_0_8px_rgba(99,102,241,0.5)]' : 'text-gray-600 dark:text-gray-400 group-hover:text-indigo-600 dark:group-hover:text-indigo-400'} strokeWidth={2.5} />
                    <span className={isActive ? 'text-indigo-600 dark:text-indigo-400 font-semibold drop-shadow-[0_0_8px_rgba(99,102,241,0.5)]' : 'text-gray-600 dark:text-gray-400 group-hover:text-gray-900 dark:group-hover:text-white'}>{item.label}</span>
                  </div>
                  {/* Active indicator — line from centre */}
                  {isActive && (
                    <motion.span
                      layoutId="nav-indicator"
                      className="absolute bottom-0 left-2 right-2 h-0.5 rounded-full bg-gradient-to-r from-indigo-500 via-purple-500 to-indigo-500"
                      initial={{ scaleX: 0 }}
                      animate={{ scaleX: 1 }}
                      transition={{ type: 'spring', stiffness: 400, damping: 30 }}
                      style={{ transformOrigin: 'center' }}
                    />
                  )}
                </Link>
              );
            })}
          </nav>

          {/* Right Section */}
          <div className="flex items-center space-x-3 md:space-x-4">
            {user ? (
              <>
                {/* Notifications */}
                <Link
                  to="/notifications"
                  className={`relative p-2.5 rounded-xl transition-colors ${location.pathname.startsWith('/notifications')
                    ? 'bg-indigo-50 dark:bg-indigo-900/20 text-indigo-600 dark:text-indigo-400'
                    : 'hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-500 dark:text-gray-400 hover:text-indigo-600 dark:hover:text-indigo-400'
                    }`}
                >
                  <motion.div
                    animate={unreadCount > 0 ? {
                      rotate: [0, -15, 15, -10, 10, -5, 5, 0],
                      transition: { duration: 0.6, ease: 'easeInOut', repeat: Infinity, repeatDelay: 4 }
                    } : {}}
                  >
                    <Bell size={20} />
                  </motion.div>
                  {unreadCount > 0 && (
                    <span className="absolute top-2 right-2.5 w-4 h-4 bg-red-500 text-white text-[10px] font-bold flex items-center justify-center rounded-full ring-2 ring-white dark:ring-gray-900 animate-pulse">
                      <span className="sr-only">{unreadCount} unread notifications</span>
                      {unreadCount > 9 ? '9+' : unreadCount}
                    </span>
                  )}
                </Link>

                {/* Theme Toggle */}
                <button
                  onClick={toggleTheme}
                  className="p-2.5 rounded-xl text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
                >
                  {theme === 'dark' ? <Sun size={20} /> : <Moon size={20} />}
                </button>

                {/* User Profile */}
                <div className="relative" ref={userMenuRef}>
                  <button
                    onClick={() => setShowUserMenu(!showUserMenu)}
                    className="flex items-center space-x-2 focus:outline-none ml-1 group"
                  >
                    <div className="w-10 h-10 bg-gradient-to-br from-indigo-500 via-purple-500 to-pink-500 p-[2px] rounded-full">
                      <div className="w-full h-full bg-white dark:bg-gray-900 rounded-full flex items-center justify-center">
                        <div className="w-full h-full bg-gradient-to-br from-indigo-500 to-purple-600 rounded-full flex items-center justify-center text-white text-sm font-bold shadow-inner">
                          {initials}
                        </div>
                      </div>
                    </div>
                  </button>

                  <AnimatePresence>
                    {showUserMenu && (
                      <motion.div
                        initial={{ opacity: 0, y: 10, scale: 0.95 }}
                        animate={{ opacity: 1, y: 0, scale: 1 }}
                        exit={{ opacity: 0, y: 10, scale: 0.95 }}
                        transition={{ duration: 0.2 }}
                        className="absolute right-0 mt-3 w-64 bg-white/90 dark:bg-gray-800/90 backdrop-blur-xl rounded-2xl shadow-2xl border border-gray-200/50 dark:border-gray-700/50 py-2 z-50 overflow-hidden"
                      >
                        <div className="px-5 py-4 border-b border-gray-100 dark:border-gray-700/50 bg-gray-50/50 dark:bg-gray-900/20">
                          <p className="text-sm font-bold text-gray-900 dark:text-white truncate">{user?.name}</p>
                          <p className="text-xs text-gray-500 dark:text-gray-400 truncate mt-0.5">{user?.email}</p>
                        </div>
                        <div className="p-2 space-y-1">
                          <Link
                            to="/profile"
                            onClick={() => setShowUserMenu(false)}
                            className="flex items-center space-x-3 px-3 py-2.5 text-sm font-medium text-gray-700 dark:text-gray-200 rounded-xl hover:bg-gray-100 dark:hover:bg-gray-700/50 transition-colors group"
                          >
                            <div className="p-1.5 bg-gray-100 dark:bg-gray-700 rounded-lg group-hover:bg-indigo-100 dark:group-hover:bg-indigo-900/30 group-hover:text-indigo-600 dark:group-hover:text-indigo-400 transition-colors">
                              <Settings size={16} />
                            </div>
                            <span>Profile Settings</span>
                          </Link>
                          <button
                            onClick={handleLogout}
                            className="w-full flex items-center space-x-3 px-3 py-2.5 text-sm font-medium text-red-600 dark:text-red-400 rounded-xl hover:bg-red-50 dark:hover:bg-red-900/10 transition-colors group"
                          >
                            <div className="p-1.5 bg-red-50 dark:bg-red-900/10 rounded-lg group-hover:bg-red-100 dark:group-hover:bg-red-900/20 transition-colors">
                              <LogOut size={16} />
                            </div>
                            <span>Sign Out</span>
                          </button>
                        </div>
                      </motion.div>
                    )}
                  </AnimatePresence>
                </div>
              </>
            ) : (
              <div className="flex items-center gap-3">
                <button
                  onClick={toggleTheme}
                  className="p-2.5 rounded-xl text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
                >
                  {theme === 'dark' ? <Sun size={20} /> : <Moon size={20} />}
                </button>
                <Link to="/login" className="px-5 py-2.5 bg-gray-900 dark:bg-white text-white dark:text-gray-900 rounded-xl font-bold text-sm hover:opacity-90 transition-opacity shadow-lg shadow-gray-500/20">
                  Sign In
                </Link>
              </div>
            )}

            {/* Mobile Menu Button */}
            <button
              className="lg:hidden p-2.5 text-gray-500 hover:text-gray-900 dark:hover:text-white transition-colors"
              onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
            >
              {isMobileMenuOpen ? <X size={24} /> : <Menu size={24} />}
            </button>
          </div>
        </div>

        {/* Mobile Menu */}
        <AnimatePresence>
          {isMobileMenuOpen && (
            <motion.div
              initial={{ height: 0, opacity: 0 }}
              animate={{ height: 'auto', opacity: 1 }}
              exit={{ height: 0, opacity: 0 }}
              className="lg:hidden overflow-hidden bg-white dark:bg-gray-900 border-t border-gray-100 dark:border-gray-800 mt-2 rounded-2xl shadow-xl"
            >
              <div className="p-4 space-y-2">
                {currentNavItems.map((item) => {
                  const Icon = item.icon;
                  const isActive = location.pathname === item.path;

                  return (
                    <Link
                      key={item.path}
                      to={item.path}
                      className={`flex items-center space-x-3 px-4 py-3.5 rounded-xl text-sm font-bold transition-all ${isActive
                        ? 'bg-indigo-50 dark:bg-indigo-900/20 text-indigo-600 dark:text-indigo-400'
                        : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-800'
                        }`}
                      onClick={() => setIsMobileMenuOpen(false)}
                    >
                      <Icon size={20} strokeWidth={2} />
                      <span>{item.label}</span>
                    </Link>
                  );
                })}
                {user && (
                  <button
                    onClick={handleLogout}
                    className="w-full flex items-center space-x-3 px-4 py-3.5 rounded-xl text-sm font-bold text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/10 transition-colors"
                  >
                    <LogOut size={20} />
                    <span>Sign Out</span>
                  </button>
                )}
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </header>
  );
};

export default Header;