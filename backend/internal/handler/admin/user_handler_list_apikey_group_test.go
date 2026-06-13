package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type listUsersFilterStub struct {
	service.AdminService
	captured service.UserListFilters
}

func (s *listUsersFilterStub) ListUsers(_ context.Context, _, _ int, filters service.UserListFilters, _, _ string) ([]service.User, int64, error) {
	s.captured = filters
	return []service.User{}, 0, nil
}

func TestAdminUserListParsesAPIKeyGroupID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name  string
		query string
		want  int64
	}{
		{name: "valid id", query: "?api_key_group_id=42", want: 42},
		{name: "missing", query: "", want: 0},
		{name: "zero ignored", query: "?api_key_group_id=0", want: 0},
		{name: "negative ignored", query: "?api_key_group_id=-3", want: 0},
		{name: "non numeric ignored", query: "?api_key_group_id=abc", want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &listUsersFilterStub{AdminService: newStubAdminService()}
			router := gin.New()
			handler := NewUserHandler(stub, nil)
			router.GET("/admin/users", handler.List)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/admin/users"+tc.query, nil)
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, tc.want, stub.captured.APIKeyGroupID)
		})
	}
}
