# Chirpy 🐦

Chirpy is a lightweight social microblogging REST API built with **Go** and **PostgreSQL**. It provides the backend functionality for a simple social platform where users can register, authenticate, publish short messages (chirps), and manage their content.

The project focuses on RESTful API development, authentication, database integration, and HTTP server implementation using Go's standard library.

### Prerequisites

Make sure you have the following installed:

- [Go](https://go.dev/dl/) (version 1.27.1 or compatible)
- [PostgreSQL](https://www.postgresql.org/)
- [Git](https://git-scm.com/)

### 1. Clone the Repository

```bash
git clone https://github.com/feagy/chirpy.git
cd chirpy
```

### 2. Install Dependencies

```bash
go mod download
```

### 3. Configure Environment Variables

Create a `.env` file in the project root directory:

```env
DB_URL=postgres://username:password@localhost:5432/chirpy?sslmode=disable
JWT_SECRET=your_secure_jwt_secret
POLKA_KEY=your_polka_api_key
```

Replace the placeholder values with your PostgreSQL credentials and secure secret keys.

| Variable | Description |
|---|---|
| `DB_URL` | PostgreSQL connection string |
| `JWT_SECRET` | Secret used to sign and validate JWTs |
| `POLKA_KEY` | API key used to authenticate webhook requests |

### 4. Set Up the Database

Create a PostgreSQL database named `chirpy` (or use another database and update `DB_URL` accordingly).

Apply the SQL schema definitions located in `sql/schema/` to initialize the required database tables.

### 5. Run the Application

Start the server using:

```bash
go run .
```

The API will be available at:

```text
http://localhost:8080
```

## API Endpoints

### Health Check

| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/healthz` | Check API readiness |

### User Management & Authentication

| Method | Endpoint | Description | Authentication |
|---|---|---|---|
| POST | `/api/users` | Register a new user | No |
| PUT | `/api/users` | Update email and password | JWT |
| POST | `/api/login` | Authenticate and obtain tokens | No |
| POST | `/api/refresh` | Generate a new access token | Refresh token |
| POST | `/api/revoke` | Revoke a refresh token | Refresh token |

### Chirp Management

| Method | Endpoint | Description | Authentication |
|---|---|---|---|
| POST | `/api/chirps` | Create a new chirp | JWT |
| GET | `/api/chirps` | Retrieve all chirps | No |
| GET | `/api/chirps/{chirpId}` | Retrieve a specific chirp | No |
| DELETE | `/api/chirps/{chirpID}` | Delete a chirp | JWT (owner only) |

**Chirp Query Parameters**

The chirp retrieval endpoint supports filtering and sorting:

- `author_id`: Filter chirps by a specific user UUID.
- `sort`: Sort results in ascending (`asc`) or descending (`desc`) chronological order. Defaults to ascending.

Example:

```http
GET /api/chirps?author_id=USER_UUID&sort=desc
```

### Webhooks

| Method | Endpoint | Description |
|---|---|---|
| POST | `/api/polka/webhooks` | Process subscription upgrade events |

The webhook endpoint validates the API key and processes `user.upgraded` events to upgrade users to Chirpy Red.

### Administration & Metrics

| Method | Endpoint | Description |
|---|---|---|
| GET | `/admin/metrics` | Display file server visit statistics |
| POST | `/admin/reset` | Reset metrics and delete all users |

> **Warning:** The `/admin/reset` endpoint is destructive. It deletes all users and resets the file server visit counter. Use it only in development or testing environments.

## Example API Usage

### Register a User

```bash
curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'
```

### Log In

```bash
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }'
```

The login response includes user information, an access token, and a refresh token.

### Create a Chirp

Replace `ACCESS_TOKEN` with the JWT returned by the login endpoint.

```bash
curl -X POST http://localhost:8080/api/chirps \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ACCESS_TOKEN" \
  -d '{
    "body": "Hello, Chirpy!"
  }'
```

Chirps have a maximum length of 140 characters.

### Retrieve All Chirps

```bash
curl http://localhost:8080/api/chirps
```

### Retrieve a Specific Chirp

```bash
curl http://localhost:8080/api/chirps/CHIRP_UUID
```

### Refresh an Access Token

```bash
curl -X POST http://localhost:8080/api/refresh \
  -H "Authorization: Bearer REFRESH_TOKEN"
```

### Revoke a Refresh Token

```bash
curl -X POST http://localhost:8080/api/revoke \
  -H "Authorization: Bearer REFRESH_TOKEN"
```

