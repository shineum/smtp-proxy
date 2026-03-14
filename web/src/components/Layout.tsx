import { useState, useEffect, useCallback } from 'react';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import {
  Toolbar, ToolbarContent, ToolbarItem, ToolbarGroup,
  Dropdown, DropdownItem, DropdownList, MenuToggle,
  Tooltip,
} from '@patternfly/react-core';
import {
  BarsIcon,
  EnvelopeIcon,
  HomeAltIcon,
  UsersIcon,
  GlobeRouteIcon,
  CogIcon,
  ServerAltIcon,
  ClipboardListIcon,
} from '@patternfly/react-icons';
import { useAuth } from '../context/AuthContext';

interface NavItemDef {
  path: string;
  label: string;
  icon: React.ReactNode;
}

type ScreenSize = 'desktop' | 'tablet' | 'mobile';

function getScreenSize(): ScreenSize {
  const w = window.innerWidth;
  if (w >= 1200) return 'desktop';
  if (w >= 768) return 'tablet';
  return 'mobile';
}

export default function Layout() {
  const { me, logout, switchGroup, isAdmin } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [screenSize, setScreenSize] = useState<ScreenSize>(getScreenSize);
  const [sidebarExpanded, setSidebarExpanded] = useState(() => getScreenSize() === 'desktop');
  const [isUserMenuOpen, setIsUserMenuOpen] = useState(false);
  const [isGroupMenuOpen, setIsGroupMenuOpen] = useState(false);

  useEffect(() => {
    const handleResize = () => {
      const size = getScreenSize();
      setScreenSize(size);
      setSidebarExpanded(size === 'desktop');
    };
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  // Close sidebar on outside click (tablet/mobile overlay)
  useEffect(() => {
    if (screenSize === 'desktop' || !sidebarExpanded) return;
    const handleClick = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      if (target.closest('.app-sidebar') || target.closest('.app-header__toggle')) return;
      setSidebarExpanded(false);
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [sidebarExpanded, screenSize]);

  const handleToggle = useCallback(() => {
    setSidebarExpanded(prev => !prev);
  }, []);

  const handleNavClick = useCallback((path: string) => {
    navigate(path);
    if (screenSize !== 'desktop') {
      setSidebarExpanded(false);
    }
  }, [navigate, screenSize]);

  const navItems: NavItemDef[] = [
    { path: '/', label: 'Dashboard', icon: <HomeAltIcon /> },
    { path: '/groups', label: 'Groups', icon: <UsersIcon /> },
    ...(isAdmin ? [{ path: '/users', label: 'Users', icon: <UsersIcon /> }] : []),
    { path: '/providers', label: 'Providers', icon: <ServerAltIcon /> },
    { path: '/routing-rules', label: 'Routing Rules', icon: <GlobeRouteIcon /> },
    { path: '/messages', label: 'Messages', icon: <EnvelopeIcon /> },
    { path: '/activity', label: 'Activity Log', icon: <ClipboardListIcon /> },
    { path: '/settings', label: 'Settings', icon: <CogIcon /> },
  ];

  const currentGroup = me?.memberships?.find(
    (m) => m.group_id === me.current_group.group_id
  );

  const handleGroupSwitch = async (groupId: string) => {
    setIsGroupMenuOpen(false);
    await switchGroup(groupId);
    navigate('/');
  };

  // Sidebar state classes
  const isCollapsed = !sidebarExpanded;
  const isOverlay = sidebarExpanded && screenSize !== 'desktop';
  // Mobile: sidebar hidden unless expanded
  const isMobileHidden = screenSize === 'mobile' && !sidebarExpanded;
  // Tablet: always show icon rail (collapsed), overlay when expanded
  const showIconRail = screenSize === 'tablet' && !sidebarExpanded;

  const sidebarVisible = !isMobileHidden;

  return (
    <div className={[
      'app-layout',
      sidebarExpanded ? 'sidebar-expanded' : 'sidebar-collapsed',
      `screen-${screenSize}`,
    ].join(' ')}>

      {/* Header */}
      <header className="app-header">
        <div className="app-header__left">
          <button
            className="app-header__toggle"
            onClick={handleToggle}
            aria-label="Toggle navigation"
          >
            <BarsIcon />
          </button>
          <div className="app-header__brand">
            <EnvelopeIcon className="app-header__brand-icon" />
            <span className="app-header__brand-text">SMTP Proxy</span>
          </div>
        </div>
        <div className="app-header__right">
          <Toolbar isFullHeight>
            <ToolbarContent>
              <ToolbarGroup align={{ default: 'alignRight' }}>
                {me && me.memberships.length > 1 && (
                  <ToolbarItem>
                    <Dropdown
                      isOpen={isGroupMenuOpen}
                      onOpenChange={setIsGroupMenuOpen}
                      toggle={(toggleRef) => (
                        <MenuToggle
                          ref={toggleRef}
                          onClick={() => setIsGroupMenuOpen(!isGroupMenuOpen)}
                          isExpanded={isGroupMenuOpen}
                          className="app-header__dropdown-toggle"
                        >
                          <span className="app-header__group-icon"><UsersIcon /></span>
                          {currentGroup?.group_name || 'Select Group'}
                        </MenuToggle>
                      )}
                    >
                      <DropdownList>
                        {me.memberships.map((m) => (
                          <DropdownItem
                            key={m.group_id}
                            onClick={() => handleGroupSwitch(m.group_id)}
                            isDisabled={m.group_id === me.current_group.group_id}
                          >
                            {m.group_name} ({m.role})
                          </DropdownItem>
                        ))}
                      </DropdownList>
                    </Dropdown>
                  </ToolbarItem>
                )}
                <ToolbarItem>
                  <Dropdown
                    isOpen={isUserMenuOpen}
                    onOpenChange={setIsUserMenuOpen}
                    toggle={(toggleRef) => (
                      <MenuToggle
                        ref={toggleRef}
                        onClick={() => setIsUserMenuOpen(!isUserMenuOpen)}
                        isExpanded={isUserMenuOpen}
                        className="app-header__dropdown-toggle"
                      >
                        <span className="app-header__avatar">
                          {(me?.user.email || 'U').charAt(0)}
                        </span>
                        <span className="app-header__email">{me?.user.email || 'User'}</span>
                      </MenuToggle>
                    )}
                  >
                    <DropdownList>
                      <DropdownItem onClick={() => { setIsUserMenuOpen(false); navigate('/settings'); }}>
                        Settings
                      </DropdownItem>
                      <DropdownItem onClick={() => { setIsUserMenuOpen(false); logout(); navigate('/login'); }}>
                        Logout
                      </DropdownItem>
                    </DropdownList>
                  </Dropdown>
                </ToolbarItem>
              </ToolbarGroup>
            </ToolbarContent>
          </Toolbar>
        </div>
      </header>

      {/* Backdrop */}
      {isOverlay && (
        <div className="app-backdrop" onClick={() => setSidebarExpanded(false)} />
      )}

      {/* Sidebar */}
      {sidebarVisible && (
        <aside className={[
          'app-sidebar',
          isCollapsed ? 'app-sidebar--collapsed' : 'app-sidebar--expanded',
          isOverlay ? 'app-sidebar--overlay' : '',
          showIconRail ? 'app-sidebar--icon-rail' : '',
        ].filter(Boolean).join(' ')}>
          <nav className="app-sidebar__nav">
            {navItems.map((item) => {
              const isActive = item.path === '/'
                ? location.pathname === '/'
                : location.pathname.startsWith(item.path);
              const navLink = (
                <button
                  key={item.path}
                  className={`app-sidebar__item ${isActive ? 'app-sidebar__item--active' : ''}`}
                  onClick={() => handleNavClick(item.path)}
                >
                  <span className="app-sidebar__item-icon">{item.icon}</span>
                  <span className="app-sidebar__item-label">{item.label}</span>
                </button>
              );

              // Show tooltip on icon-only modes
              if (isCollapsed || showIconRail) {
                return (
                  <Tooltip key={item.path} content={item.label} position="right">
                    {navLink}
                  </Tooltip>
                );
              }
              return navLink;
            })}
          </nav>
        </aside>
      )}

      {/* Main content */}
      <main className="app-content">
        <Outlet />
      </main>
    </div>
  );
}
