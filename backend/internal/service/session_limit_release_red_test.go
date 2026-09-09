package service

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReleaseAccountSession_ExposesFailedRegistrationReleaseOperation(t *testing.T) {
	// Given
	typeOfService := reflect.TypeOf((*GatewayService)(nil))

	// When
	method, ok := typeOfService.MethodByName("ReleaseAccountSession")

	// Then
	require.True(t, ok, "GatewayService must expose ReleaseAccountSession for failed max-session registrations")
	require.Equal(t, 4, method.Type.NumIn())
}
