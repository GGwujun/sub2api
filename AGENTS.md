# AGENTS.md - AI Agent Coding Guidelines

This document provides essential information for AI agents working on the Sub2API codebase.

## Tech Stack

| Component | Technology |
|-----------|------------|
| Backend | Go 1.25.7, Gin, Ent ORM, Wire DI |
| Frontend | Vue 3.4+, Vite 5+, TypeScript, Pinia, TailwindCSS |
| Database | PostgreSQL 15+ |
| Cache/Queue | Redis 7+ |
| Package Manager | Backend: go modules, Frontend: **pnpm** (NOT npm) |

## Build / Lint / Test Commands

### Backend (Go)

```bash
cd backend

# Build
make build                    # Build server binary
go build -tags embed -o bin/server ./cmd/server

# Run development
go run ./cmd/server

# Code generation (after modifying ent/schema)
go generate ./ent
go generate ./cmd/server
# COMMIT the generated files in ent/

# Testing
make test                     # Run all tests + lint
make test-unit                # Run unit tests only
make test-integration         # Run integration tests only
go test -tags=unit ./...      # Unit tests
go test -tags=integration ./...  # Integration tests
go test -v ./path/to/package  # Run specific package
go test -v ./path/to/file_test.go  # Run specific file
go test -v -run TestSpecificFunction ./path/to/file_test.go  # Run single test

# Linting
golangci-lint run ./...       # Run all linters (v2.7)
```

### Frontend (Vue 3)

```bash
cd frontend

# Dependency management (MUST use pnpm, not npm)
pnpm install                  # Install dependencies
pnpm install --frozen-lockfile  # CI mode (requires pnpm-lock.yaml committed)

# Development
pnpm dev                     # Start dev server (port 3000 by default)
pnpm build                    # Build for production (outputs to ../backend/internal/web/dist/)

# Type checking
pnpm run typecheck            # vue-tsc --noEmit

# Linting
pnpm run lint:check           # Check lint errors
pnpm run lint                 # Check and fix lint errors

# Testing (Vitest)
pnpm test                     # Interactive test mode
pnpm test:run                # Run tests once
pnpm test -- -r               # Run specific test file
pnpm test -- --grep "test name"  # Run tests matching name pattern
pnpm test:coverage           # Generate coverage report
```

### Root Makefile

```bash
# From project root
make build                    # Build both backend and frontend
make build-backend            # Build backend only
make build-frontend           # Build frontend only

make test                     # Run all tests (backend + frontend)
make test-backend             # Run backend tests only
make test-frontend            # Run frontend lint + typecheck only
```

## Backend Code Style (Go)

### Import Organization
```go
// Order: 1. std lib (sorted), 2. third-party (sorted), 3. internal packages
import (
    "context"
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"
    "go.uber.org/zap"

    "github.com/Wei-Shaw/sub2api/internal/config"
    "github.com/Wei-Shaw/sub2api/internal/service"
)
```

### Naming Conventions
- **Packages**: Lowercase, single word (`handler`, `service`, `repository`)
- **Exported types**: PascalCase (`AuthHandler`, `RegisterRequest`, `User`)
- **Exported functions/methods**: PascalCase (`NewAuthHandler`, `HandleLogin`)
- **Internal/unexported**: camelCase (`handleInternal`, `validateUser`)
- **Interfaces**: Usually defined in service layer, named with `er` suffix if appropriate

### Handler Patterns
```go
// Handlers receive services, NOT repositories
type AuthHandler struct {
    cfg         *config.Config
    authService *service.AuthService
}

func NewAuthHandler(cfg *config.Config, authService *service.AuthService) *AuthHandler {
    return &AuthHandler{cfg: cfg, authService: authService}
}

// Gin handler methods
func (h *AuthHandler) Login(c *gin.Context) {
    var req LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, "Invalid request")
        return
    }

    // Use service for business logic
    user, err := h.authService.Login(c.Request.Context(), req.Email, req.Password)
    if err != nil {
        response.Unauthorized(c, "Invalid credentials")
        return
    }

    response.Success(c, AuthResponse{User: user})
}
```

### Error Handling
- Use `slog.Error()` for structured logging (field-based)
- Import `github.com/Wei-Shaw/sub2api/internal/pkg/response` for API responses
- Response helpers: `response.Success()`, `response.BadRequest()`, `response.Unauthorized()`, `response.InternalError()`

### Service Layer
- **Services** handle business logic and coordinate repositories
- **Handlers** depend on services (NEVER directly on repositories)
- **Repositories** handle data access (Ent ORM)

### Ent ORM
- Schema definitions in `backend/ent/schema/*.go`
- After schema changes: `go generate ./ent`
- Generated code in `backend/ent/` must be committed

### Architecture Enforcement (golangci-lint)
- Service layer CANNOT import `internal/repository` or `gorm`
- Handler layer CANNOT import `internal/repository` or `gorm` or `redis`
- Exceptions exist in `service/ops_*.go` files (see `.golangci.yml`)

## Frontend Code Style (Vue 3 + TypeScript)

### Component Structure
```vue
<script setup lang="ts">
// Imports from local modules first, then third-party
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from './BaseDialog.vue'

// Props and emits with TypeScript interfaces
interface Props {
  show: boolean
  title: string
  message: string
}

interface Emits {
  (e: 'confirm'): void
  (e: 'cancel'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

// Composition API
const { t } = useI18n()
const isLoading = ref(false)
const canConfirm = computed(() => props.show && !isLoading.value)

// Methods
function handleConfirm() {
  emit('confirm')
}
</script>

<template>
  <!-- Use @ for events, : for bindings, :class for conditional classes -->
  <div :class="{ active: show }">
    <p class="text-sm">{{ message }}</p>
    <button @click="handleConfirm" :disabled="isLoading">
      {{ t('common.confirm') }}
    </button>
  </div>
</template>

<style scoped>
/* Minimal CSS - prefer Tailwind utility classes */
</style>
```

### Import Organization
```typescript
// 1. Vue ecosystem
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'

// 2. Third-party libraries
import axios from 'axios'

// 3. Local modules (use @/ path alias)
import { useAuthStore } from '@/stores'
import type { User } from '@/types'
```

### Store Pattern (Pinia)
```typescript
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  // State
  const user = ref<User | null>(null)
  const token = ref<string | null>(null)

  // Computed
  const isAuthenticated = computed(() => !!token.value && !!user.value)

  // Actions
  function login(credentials: LoginRequest) {
    // async operation
  }

  function logout() {
    user.value = null
    token.value = null
  }

  return { user, token, isAuthenticated, login, logout }
})
```

### API Calls
```typescript
import { apiClient } from '@/api/client'

// Standard API response format
interface ApiResponse<T> {
  code: number
  message: string
  data: T
}

// Use apiClient (handles auth, token refresh, errors)
async function fetchUsers(): Promise<User[]> {
  const response = await apiClient.get<User[]>('/users')
  return response.data
}
```

### i18n (Internationalization)
```typescript
import { useI18n } from 'vue-i18n'

const { t, locale } = useI18n()

// Usage
const message = t('common.save')
const greeting = t('hello', { name: 'World' })
```

### Routing
```typescript
import { useRouter, useRoute } from 'vue-router'

const router = useRouter()
const route = useRoute()

// Navigation
router.push({ name: 'users' })
router.push({ path: '/users', query: { page: 1 } })
router.back()

// Route params
const userId = route.params.id as string
```

### Path Aliases
- `@/` → `src/`
- Example: `import { User } from '@/types'`

### File Naming
- **Components**: PascalCase.vue (`ConfirmDialog.vue`, `DataTable.vue`)
- **Composables**: camelCase.ts starting with `use` (`useForm.ts`, `useRouter.ts`)
- **Utilities**: camelCase.ts (`format.ts`, `sanitize.ts`)
- **Types**: camelCase.ts or `index.ts` (`types/index.ts`)

## Critical Gotchas

### Frontend - Package Management
1. **MUST use pnpm, NOT npm** - mixing them causes node_modules conflicts
2. **Always commit `pnpm-lock.yaml`** - CI uses `pnpm install --frozen-lockfile`
3. After `package.json` changes: run `pnpm install` and commit the updated lock file

### Backend - Ent Schema
1. After editing `ent/schema/*.go`, MUST run `go generate ./ent`
2. Generated code in `ent/` must be committed to git

### Backend - Testing
1. When adding methods to interfaces, all test stubs implementing those interfaces must be updated
2. Use build tags: `//go:build unit` or `//go:build integration` for test file categorization

### PowerShell - bcrypt Hash Handling
bcrypt hashes contain `$` which PowerShell interprets as variables. Use SQL files:
```bash
# Wrong (PowerShell eats the $):
psql -c "INSERT INTO users ... VALUES ('$2a$10$xxx...')"

# Correct (SQL file):
echo "INSERT INTO users ... VALUES ('\$2a\$10\$...')" > temp.sql
psql -f temp.sql
```

## Project Structure

```
sub2api/
├── backend/
│   ├── cmd/server/          # Application entry point
│   ├── ent/                 # Ent ORM generated code (schema in ent/schema/)
│   ├── internal/
│   │   ├── handler/         # HTTP handlers (depends on services)
│   │   ├── service/         # Business logic (depends on repositories)
│   │   ├── repository/      # Data access (Ent ORM)
│   │   ├── domain/          # Domain models
│   │   ├── config/          # Configuration
│   │   └── server/          # Gin server setup
│   └── migrations/          # Database migrations
│
├── frontend/
│   ├── src/
│   │   ├── api/             # API client functions
│   │   ├── stores/          # Pinia stores
│   │   ├── components/      # Vue components
│   │   ├── composables/     # Reusable composition functions
│   │   ├── views/           # Page-level components
│   │   ├── router/          # Vue Router config
│   │   ├── i18n/            # Internationalization
│   │   ├── types/           # TypeScript types
│   │   └── utils/           # Utility functions
│   └── package.json         # Dependencies (use pnpm)
│
└── deploy/                  # Deployment configs (docker-compose, install scripts)
```

## Common Patterns Reference

### Backend Error Response
```go
import "github.com/Wei-Shaw/sub2api/internal/pkg/response"

response.BadRequest(c, "Invalid email format")
response.Unauthorized(c, "Invalid credentials")
response.NotFound(c, "User not found")
response.InternalError(c, "Database error")
response.Success(c, data)
```

### Frontend Async with Loading
```typescript
import { useAppStore } from '@/stores'

const appStore = useAppStore()

try {
  await appStore.withLoading(async () => {
    await fetchData()
  })
  appStore.showSuccess('Data loaded successfully')
} catch (error) {
  appStore.showError('Failed to load data')
}
```

### Frontend Composable Pattern
```typescript
import { ref, watch } from 'vue'

export function useDebouncedSearch<T>(
  searchFn: (query: string) => Promise<T[]>,
  delay = 300
) {
  const results = ref<T[]>([])
  const query = ref('')
  const isLoading = ref(false)

  let timeoutId: ReturnType<typeof setTimeout> | null = null

  watch(query, (newQuery) => {
    if (timeoutId) clearTimeout(timeoutId)

    timeoutId = setTimeout(async () => {
      isLoading.value = true
      try {
        results.value = await searchFn(newQuery)
      } finally {
        isLoading.value = false
      }
    }, delay)
  })

  return { results, query, isLoading }
}
```
