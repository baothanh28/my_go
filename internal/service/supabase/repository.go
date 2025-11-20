package supabase

import (
	"fmt"

	"myapp/internal/pkg/database"
)

// Repository handles database operations for users in supabase_login domain
type Repository struct {
	db *database.Database
}

// NewRepository creates a new repository
func NewRepository(db *database.Database) *Repository {
	return &Repository{db: db}
}

// GetByEmail retrieves a user by email
func (r *Repository) GetByEmail(email string) (*User, error) {
	var user User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &user, nil
}

// Create inserts a new user
func (r *Repository) Create(user *User) error {
	if err := r.db.Create(user).Error; err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetUserRoleAndPermissions returns role and combined permissions for a Supabase user id
func (r *Repository) GetUserRoleAndPermissions(userID string) (roleName string, permissions []string, err error) {
	// Use a single SQL with CTEs similar to the Edge Function example
	// Note: This uses the same table names as provided by the user's index.ts example
	sql := `
      WITH user_role_info AS (
          SELECT t2.name AS role_name, t1.role_id
          FROM public.sp_user_roles t1
          JOIN public.sp_roles t2 ON t1.role_id = t2.id
          WHERE t1.user_id = ?
      ),
      role_permissions AS (
          SELECT permission
          FROM public.sp_role_permissions
          WHERE role_id = (SELECT role_id FROM user_role_info LIMIT 1)
      ),
      user_permissions AS (
          SELECT permission
          FROM public.sp_user_permissions
          WHERE user_id = ?
      )
      SELECT
          (SELECT role_name FROM user_role_info LIMIT 1) AS role_name,
          COALESCE(ARRAY(SELECT permission FROM role_permissions), ARRAY[]::text[]) AS role_perms_array,
          COALESCE(ARRAY(SELECT permission FROM user_permissions), ARRAY[]::text[]) AS user_perms_array;`

	type row struct {
		RoleName       *string
		RolePermsArray []string
		UserPermsArray []string
	}

	var result row
	// GORM doesn't directly map array return types to struct fields unless using sql.Scanner. Use Raw and Scan
	if err := r.db.Raw(sql, userID, userID).Scan(&result).Error; err != nil {
		return "", nil, fmt.Errorf("failed to query permissions: %w", err)
	}

	// Merge permissions, deduplicate
	unique := map[string]struct{}{}
	for _, p := range result.RolePermsArray {
		unique[p] = struct{}{}
	}
	for _, p := range result.UserPermsArray {
		unique[p] = struct{}{}
	}
	merged := make([]string, 0, len(unique))
	for k := range unique {
		merged = append(merged, k)
	}

	rname := ""
	if result.RoleName != nil {
		rname = *result.RoleName
	}
	return rname, merged, nil
}
