import { useState } from 'react';
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
  NetworkIcon,
  GlobeRouteIcon,
  ListAltIcon,
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

export default function Layout() {
  const { me, logout, switchGroup, isSystemAdmin, isAdmin } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [isSidebarOpen, setIsSidebarOpen] = useState(true);
  const [isUserMenuOpen, setIsUserMenuOpen] = useState(false);
  const [isGroupMenuOpen, setIsGroupMenuOpen] = useState(false);

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

  const masthead = (
    <Masthead>
      <MastheadToggle>
        <PageToggleButton
          variant="plain"
          aria-label="Global navigation"
          isSidebarOpen={isSidebarOpen}
          onSidebarToggle={() => setIsSidebarOpen(!isSidebarOpen)}
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
    <PageSidebar isSidebarOpen={isSidebarOpen}>
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
                onClick={() => navigate(item.path)}
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

  return (
    <Page header={masthead} sidebar={sidebar}>
      <Outlet />
    </Page>
  );
}
