-- Fix: RLS policy on users table blocks INSERT because the new user
-- has no group_members entry yet at insert time.
-- Replace ALL policy with SELECT/UPDATE/DELETE only.
-- INSERT permission is controlled at the application level (requireGroupRole).

DROP POLICY IF EXISTS user_group_isolation ON users;

CREATE POLICY user_group_isolation ON users
    FOR SELECT
    USING (EXISTS (
        SELECT 1 FROM group_members gm
        WHERE gm.user_id = users.id
          AND gm.group_id = current_setting('app.current_group_id', true)::UUID
    ));

CREATE POLICY user_group_isolation_update ON users
    FOR UPDATE
    USING (EXISTS (
        SELECT 1 FROM group_members gm
        WHERE gm.user_id = users.id
          AND gm.group_id = current_setting('app.current_group_id', true)::UUID
    ));

CREATE POLICY user_group_isolation_delete ON users
    FOR DELETE
    USING (EXISTS (
        SELECT 1 FROM group_members gm
        WHERE gm.user_id = users.id
          AND gm.group_id = current_setting('app.current_group_id', true)::UUID
    ));

-- Allow unrestricted INSERT (app-level auth handles permission).
CREATE POLICY user_allow_insert ON users
    FOR INSERT
    WITH CHECK (true);
