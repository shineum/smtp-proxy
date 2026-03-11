DROP POLICY IF EXISTS user_group_isolation ON users;
DROP POLICY IF EXISTS user_group_isolation_update ON users;
DROP POLICY IF EXISTS user_group_isolation_delete ON users;
DROP POLICY IF EXISTS user_allow_insert ON users;

CREATE POLICY user_group_isolation ON users
    FOR ALL
    USING (EXISTS (
        SELECT 1 FROM group_members gm
        WHERE gm.user_id = users.id
          AND gm.group_id = current_setting('app.current_group_id', true)::UUID
    ));
