package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

var usageProjectHeaderCandidates = []string{
	"X-Sub2API-Project",
	"X-Project",
	"X-Project-Name",
}

var usageProjectJSONPaths = []string{
	"metadata.project",
	"metadata.project_name",
	"metadata.project_id",
	"project",
	"project_id",
}

// ExtractUsageProject extracts optional project attribution from explicit
// client-provided metadata. Missing or malformed project metadata is ignored.
func ExtractUsageProject(c *gin.Context, body []byte) string {
	if c != nil && c.Request != nil {
		for _, header := range usageProjectHeaderCandidates {
			if project := service.NormalizeUsageProjectLabel(c.GetHeader(header)); project != "" {
				return project
			}
		}
	}
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	for _, path := range usageProjectJSONPaths {
		value := gjson.GetBytes(body, path)
		if !value.Exists() || value.Type != gjson.String {
			continue
		}
		if project := service.NormalizeUsageProjectLabel(strings.TrimSpace(value.String())); project != "" {
			return project
		}
	}
	return ""
}
