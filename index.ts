// File: supabase/functions/get-user-permissions/index.ts
import { serve } from "https://deno.land/std@0.168.0/http/server.ts";
import { createClient } from "https://esm.sh/@supabase/supabase-js@2";
import { Pool } from "https://deno.land/x/postgres@v0.17.0/mod.ts";
// Lấy chuỗi kết nối database trực tiếp từ biến môi trường
const DATABASE_URL = Deno.env.get("SUPABASE_DB_URL");
// Khởi tạo Pool kết nối PostgreSQL
const pool = new Pool(DATABASE_URL, 3, true);
serve(async (req)=>{
  const connection = await pool.connect(); // Mở kết nối database
  try {
    // --- 1. Xác thực Người dùng (Sử dụng Service Client như trước) ---
    const supabaseUrl = Deno.env.get("SUPABASE_URL");
    const supabaseServiceKey = Deno.env.get("SUPABASE_SERVICE_ROLE_KEY");
    const supabase = createClient(supabaseUrl, supabaseServiceKey, {
      auth: {
        persistSession: false
      }
    });
    const authHeader = req.headers.get("Authorization");
    if (!authHeader) return new Response("Missing Authorization header", {
      status: 401
    });
    const token = authHeader.replace("Bearer ", "").trim();
    const { data: { user }, error: userError } = await supabase.auth.getUser(token);
    if (userError || !user) return new Response("Unauthorized", {
      status: 401
    });
    const userId = user.id;
    // --- 2. Truy vấn Quyền bằng Kết nối Trực tiếp (Raw SQL) ---
    // SQL để lấy vai trò, quyền của vai trò, và quyền cá nhân trong MỘT truy vấn
    const query = `
      WITH user_role_info AS (
          SELECT
              t2.name AS role_name,
              t1.role_id
          FROM
              public.sp_user_roles t1
          JOIN
              public.sp_roles t2 ON t1.role_id = t2.id
          WHERE
              t1.user_id = $1
      ),
      role_permissions AS (
          SELECT
              permission
          FROM
              public.sp_role_permissions
          WHERE
              role_id = (SELECT role_id FROM user_role_info LIMIT 1)
      ),
      user_permissions AS (
          SELECT
              permission
          FROM
              public.sp_user_permissions
          WHERE
              user_id = $1
      )
      SELECT
          (SELECT role_name FROM user_role_info LIMIT 1) AS role_name,
          COALESCE(ARRAY(SELECT permission FROM role_permissions), ARRAY[]::text[]) AS role_perms_array,
          COALESCE(ARRAY(SELECT permission FROM user_permissions), ARRAY[]::text[]) AS user_perms_array;
    `;
    const result = await connection.queryObject(query, [
      userId
    ]);
    const row = result.rows[0];
    // --- 3. Kết hợp và Phản hồi ---
    const roleName = row?.role_name || null;
    const rolePermissions = row?.role_perms_array || [];
    const userPermissions = row?.user_perms_array || [];
    // Gộp tất cả quyền và loại bỏ các quyền trùng lặp
    const allPermissions = Array.from(new Set([
      ...rolePermissions,
      ...userPermissions
    ]));
    const responseData = {
      role: roleName,
      permissions: allPermissions
    };
    return new Response(JSON.stringify(responseData), {
      headers: {
        "Content-Type": "application/json"
      },
      status: 200
    });
  } catch (err) {
    console.error("Edge Function Error:", err);
    return new Response(JSON.stringify({
      error: "Server error during permission retrieval"
    }), {
      status: 500
    });
  } finally{
    // Rất quan trọng: Luôn giải phóng kết nối
    connection.release();
  }
});
