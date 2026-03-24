import { useState, useEffect, useCallback, useRef } from 'react';
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
  BuildingIcon,
  UsersIcon,
  GlobeRouteIcon,
  CogIcon,
  ServerAltIcon,
  ClipboardListIcon,
  SignOutAltIcon,
  OutlinedClockIcon,
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

interface UserProfilePopoverProps {
  me: import('../types/api').MeResponse | null;
  currentGroup: import('../types/api').Membership | undefined;
  isOpen: boolean;
  onToggle: () => void;
  onClose: () => void;
  onSettings: () => void;
  onLogout: () => void;
}

function UserProfilePopover({ me, currentGroup, isOpen, onToggle, onClose, onSettings, onLogout }: UserProfilePopoverProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen) return;
    const handleClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [isOpen, onClose]);

  const email = me?.user.email || 'User';
  const initial = email.charAt(0).toUpperCase();
  const role = currentGroup?.role || me?.current_group.role || '';
  const groupName = currentGroup?.group_name || '';

  return (
    <div className="profile-popover" ref={ref}>
      <button className="profile-trigger" onClick={onToggle} type="button">
        <span className="profile-trigger__avatar">{initial}</span>
        <span className="profile-trigger__email">{email}</span>
      </button>
      {isOpen && (
        <div className="profile-panel">
          <div className="profile-panel__header">
            <div className="profile-panel__avatar-lg">{initial}</div>
            <div className="profile-panel__info">
              {me?.user.display_name && (
                <div className="profile-panel__name">{me.user.display_name}</div>
              )}
              <div className="profile-panel__email">{email}</div>
              {groupName && (
                <div className="profile-panel__group">{groupName} &middot; {role}</div>
              )}
            </div>
          </div>
          <div className="profile-panel__divider" />
          <div className="profile-panel__actions">
            <button className="profile-panel__action" onClick={onSettings} type="button">
              <CogIcon /> Settings
            </button>
            <button className="profile-panel__action profile-panel__action--logout" onClick={onLogout} type="button">
              <SignOutAltIcon /> Logout
            </button>
          </div>
        </div>
      )}
    </div>
  );
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
    { path: '/groups', label: 'Groups', icon: <BuildingIcon /> },
    ...(isAdmin ? [{ path: '/users', label: 'Users', icon: <UsersIcon /> }] : []),
    { path: '/providers', label: 'Providers', icon: <ServerAltIcon /> },
    { path: '/routing-rules', label: 'Routing Rules', icon: <GlobeRouteIcon /> },
    { path: '/domain-rate-limits', label: 'Rate Limits', icon: <OutlinedClockIcon /> },
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
                  <UserProfilePopover
                    me={me}
                    currentGroup={currentGroup}
                    isOpen={isUserMenuOpen}
                    onToggle={() => setIsUserMenuOpen(prev => !prev)}
                    onClose={() => setIsUserMenuOpen(false)}
                    onSettings={() => { setIsUserMenuOpen(false); navigate('/settings'); }}
                    onLogout={() => { setIsUserMenuOpen(false); logout(); navigate('/login'); }}
                  />
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
