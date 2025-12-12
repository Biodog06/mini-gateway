### 1. **请求定义**
```
r = sub, obj, act
```
- **解释**：请求由三个元素组成：
    - `sub`：主体（例如用户或角色）。
    - `obj`：访问的资源（例如 URL 或 API 端点）。
    - `act`：执行的操作（例如 GET、POST）。
- **示例**：`r = admin, /api/v1/user, GET` 表示 `admin` 角色尝试对 `/api/v1/user` 执行 `GET` 操作。

---

### 2. **策略定义**
```
p = sub, obj, act
```
- **解释**：策略也是由三个元素组成，定义权限：
    - `sub`：适用策略的主体（用户或角色）。
    - `obj`：策略管理的资源。
    - `act`：允许的操作。
- **示例**：`p, admin, /api/v1/user, GET` 表示 `admin` 角色被允许对 `/api/v1/user` 执行 `GET` 操作。

---

### 3. **角色定义**
```
g = _, _
```
- **解释**：定义角色继承或角色到用户的映射。
    - `g, admin, user` 表示 `admin` 角色继承 `user` 角色的权限（或者 `admin` 用户被分配了 `user` 角色，具体取决于系统实现）。
    - `_` 表示占位符，灵活表示主体和角色。
- **示例**：`g, admin, user` 意味着 `admin` 拥有 `user` 角色的所有权限。

---

### 4. **策略效果**
```
e = some(where (p.eft == allow))
```
- **解释**：定义策略的评估方式：
    - `p.eft == allow` 表示策略必须明确允许操作（默认情况下，未指定的操作被拒绝）。
    - `some(where ...)` 表示只要至少有一个匹配的策略允许请求，访问就被允许。
- **实际应用**：如果有任何匹配的策略允许请求，访问被授予；否则，访问被拒绝。

---

### 5. **匹配器**
```
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
```
- **解释**：匹配器决定请求 (`r`) 是否匹配策略 (`p`)：
    - `g(r.sub, p.sub)`：检查请求的主体 (`r.sub`) 是否拥有或继承策略中指定的角色 (`p.sub`)。
    - `r.obj == p.obj`：确保请求的资源与策略的资源匹配。
    - `r.act == p.act`：确保请求的操作与策略的操作匹配。
- **实际应用**：只有当请求的主体、资源和操作与策略完全匹配时，请求才被允许。

---

### 6. **具体策略和角色分配**
提供的策略和角色映射如下：
```
p, admin, /api/v1/user, GET
p, admin, /api/v1/user, POST
p, admin, /api/v1/order, GET
p, admin, /api/v1/order, POST
p, user, /api/v1/user, GET
g, admin, user
```
- **策略**：
    - `admin` 角色可以：
        - 对 `/api/v1/user` 执行 `GET` 和 `POST`。
        - 对 `/api/v1/order` 执行 `GET` 和 `POST`。
    - `user` 角色可以：
        - 对 `/api/v1/user` 执行 `GET`。
- **角色继承**：
    - `g, admin, user`：`admin` 角色继承 `user` 角色的权限，因此 `admin` 也能对 `/api/v1/user` 执行 `GET`（除了自己的权限外）。

---

### 7. **工作原理**
- 当收到请求（例如 `r = admin, /api/v1/user, GET`）：
    1. 匹配器检查 `admin` 是否拥有所需角色或继承角色：
        - 由于存在 `g, admin, user`，`admin` 继承了 `user` 的权限。
        - 因此会考虑 `admin` 和 `user` 的策略。
    2. 匹配器验证资源 (`/api/v1/user`) 和操作 (`GET`) 是否匹配任何策略：
        - 匹配 `p, admin, /api/v1/user, GET`（直接权限）。
        - 也匹配 `p, user, /api/v1/user, GET`（通过继承）。
    3. 策略效果 (`e = some(where (p.eft == allow))`) 确认至少有一个策略允许请求，因此访问被授予。

- **示例场景**：
    - 请求：`r = admin, /api/v1/order, POST`
        - 匹配 `p, admin, /api/v1/order, POST` → **允许**。
    - 请求：`r = user, /api/v1/user, GET`
        - 匹配 `p, user, /api/v1/user, GET` → **允许**。
    - 请求：`r = user, /api/v1/order, GET`
        - 无匹配策略 → **拒绝**。
    - 请求：`r = admin, /api/v1/user, GET`
        - 匹配 `p, admin, /api/v1/user, GET` 和 `p, user, /api/v1/user, GET`（通过继承） → **允许**。

---

### 8. **总结**
- **RBAC 结构**：系统通过角色（`admin`、`user`）为资源和操作分配权限。
- **角色继承**：`admin` 继承 `user` 的权限，因此拥有 `user` 的所有权限加上自己的权限。
- **访问控制**：只有当至少有一个匹配的策略明确允许请求时，访问才被授予。
- **应用场景**：这种配置常用于 API 访问控制，`admin` 角色有更广泛的权限（例如管理用户和订单），而 `user` 角色权限有限（例如仅查看自己的用户数据）。