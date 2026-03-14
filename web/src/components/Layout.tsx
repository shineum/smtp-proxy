import { useState, useEffect, useCallback } from 'react';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import {
  Page, Masthead, MastheadMain, MastheadBrand, MastheadContent, MastheadToggle,
  PageSidebar, PageSidebarBody,
  Nav, NavItem, NavList,
  Toolbar, ToolbarContent, ToolbarItem, ToolbarGroup,
  Dropdown, DropdownItem, DropdownList, MenuToggle,
  PageToggleButton,
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
  const [isExpanded, setIsExpanded] = useState(() => getScreenSize() === 'desktop');
  const [isUserMenuOpen, setIsUserMenuOpen] = useState(false);
  const [isGroupMenuOpen, setIsGroupMenuOpen] = useState(false);

  // Track screen size changes
  useEffect(() => {
    const handleResize = () => {
      const newSize = getScreenSize();
      setScreenSize(newSize);
      // Auto-expand on desktop, auto-collapse on resize to smaller
      if (newSize === 'desktop') {
        setIsExpanded(true);
      } else {
        setIsExpanded(false);
      }
    };
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  // Close expanded sidebar when clicking outside (tablet/mobile)
  useEffect(() => {
    if (!isExpanded || screenSize === 'desktop') return;

    const handleClickOutside = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      if (target.closest('.pf-v5-c-page__sidebar')) return;
      if (target.closest('.pf-v5-c-masthead__toggle')) return;
      setIsExpanded(false);
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [isExpanded, screenSize]);

  const handleToggle = useCallback(() => {
    if (screenSize === 'mobile') {
      // Mobile: toggle visibility (hidden <-> expanded overlay)
      setIsExpanded(prev => !prev);
    } else {
      // Desktop & Tablet: toggle expanded <-> collapsed (icon-only)
      setIsExpanded(prev => !prev);
    }
  }, [screenSize]);

  const handleNavItemClick = useCallback((path: string) => {
    navigate(path);
    // On tablet: collapse back to icon-only after nav
    // On mobile: hide sidebar after nav
    if (screenSize !== 'desktop') {
      setIsExpanded(false);
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

  // Sidebar is always visible on desktop/tablet, only hidden on mobile when not expanded
  const isSidebarVisible = screenSize !== 'mobile' || isExpanded;

  const masthead = (
    <Masthead>
      <MastheadToggle>
        <PageToggleButton
          variant="plain"
          aria-label="Global navigation"
          isSidebarOpen={isExpanded}
          onSidebarToggle={handleToggle}
        >
          <BarsIcon />
        </PageToggleButton>
      </MastheadToggle>
      <MastheadMain>
        <MastheadBrand data-codemods className="masthead-brand">
          <EnvelopeIcon className="masthead-brand-icon" />
          <span className="masthead-brand-text">SMTP Proxy</span>
        </MastheadBrand>
      </MastheadMain>
      <MastheadContent>
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
                      >
                        <span className="masthead-group-icon"><UsersIcon /></span>
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
                    >
                      <span className="masthead-user-avatar">
                        {(me?.user.email || 'U').charAt(0)}
                      </span>
                      {me?.user.email || 'User'}
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
      </MastheadContent>
    </Masthead>
  );

  const sidebar = (
    <PageSidebar isSidebarOpen={isSidebarVisible}>
      <PageSidebarBody>
        <Nav>
          <NavList>
            {navItems.map((item) => (
              <NavItem
                key={item.path}
                isActive={
                  item.path === '/'
                    ? location.pathname === '/'
                    : location.pathname.startsWith(item.path)
                }
                onClick={() => handleNavItemClick(item.path)}
              >
                <span className="nav-item-content">
                  <span className="nav-item-icon">{item.icon}</span>
                  <span className="nav-item-label">{item.label}</span>
                </span>
              </NavItem>
            ))}
          </NavList>
        </Nav>
      </PageSidebarBody>
    </PageSidebar>
  );

  // Build page CSS classes
  const isOverlay = isExpanded && screenSize !== 'desktop';
  const isCollapsed = !isExpanded && screenSize !== 'mobile';
  const pageClasses = [
    isOverlay ? 'sidebar-overlay-active' : '',
    isCollapsed ? 'sidebar-collapsed' : '',
    `screen-${screenSize}`,
  ].filter(Boolean).join(' ');

  return (
    <Page
      header={masthead}
      sidebar={sidebar}
      className={pageClasses}
    >
      <Outlet />
    </Page>
  );
}
