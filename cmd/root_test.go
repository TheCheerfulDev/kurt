package cmd

import (
	"testing"

	"github.com/kurt/kurt/pkg/model"
	"github.com/stretchr/testify/assert"
)

// ---------- parseExcludeKinds tests ----------

func TestParseExcludeKinds_Empty(t *testing.T) {
	result := parseExcludeKinds(nil)
	assert.Nil(t, result)
}

func TestParseExcludeKinds_EmptySlice(t *testing.T) {
	result := parseExcludeKinds([]string{})
	assert.Nil(t, result)
}

func TestParseExcludeKinds_SingleKind(t *testing.T) {
	result := parseExcludeKinds([]string{"replicaset"})
	assert.True(t, result[model.KindReplicaSet])
	assert.Len(t, result, 1)
}

func TestParseExcludeKinds_Aliases(t *testing.T) {
	tests := []struct {
		input    string
		expected model.ResourceKind
	}{
		{"deploy", model.KindDeployment},
		{"deployment", model.KindDeployment},
		{"sts", model.KindStatefulSet},
		{"statefulset", model.KindStatefulSet},
		{"rs", model.KindReplicaSet},
		{"replicaset", model.KindReplicaSet},
		{"pod", model.KindPod},
		{"svc", model.KindService},
		{"service", model.KindService},
		{"vs", model.KindVirtualService},
		{"virtualservice", model.KindVirtualService},
		{"gw", model.KindGateway},
		{"gateway", model.KindGateway},
		{"ing", model.KindIngress},
		{"ingress", model.KindIngress},
		{"ap", model.KindAuthorizationPolicy},
		{"authorizationpolicy", model.KindAuthorizationPolicy},
		{"dr", model.KindDestinationRule},
		{"destinationrule", model.KindDestinationRule},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseExcludeKinds([]string{tt.input})
			assert.True(t, result[tt.expected], "expected %s to map to %s", tt.input, tt.expected)
		})
	}
}

func TestParseExcludeKinds_CaseInsensitive(t *testing.T) {
	result := parseExcludeKinds([]string{"ReplicaSet"})
	assert.True(t, result[model.KindReplicaSet])
}

func TestParseExcludeKinds_MixedCase(t *testing.T) {
	result := parseExcludeKinds([]string{"DEPLOY", "Svc"})
	assert.True(t, result[model.KindDeployment])
	assert.True(t, result[model.KindService])
	assert.Len(t, result, 2)
}

func TestParseExcludeKinds_UnknownKindsIgnored(t *testing.T) {
	result := parseExcludeKinds([]string{"unknown", "nonsense"})
	assert.Nil(t, result, "unknown kinds should result in nil map")
}

func TestParseExcludeKinds_MixedValidInvalid(t *testing.T) {
	result := parseExcludeKinds([]string{"replicaset", "unknown", "pod"})
	assert.Len(t, result, 2)
	assert.True(t, result[model.KindReplicaSet])
	assert.True(t, result[model.KindPod])
}

func TestParseExcludeKinds_Whitespace(t *testing.T) {
	result := parseExcludeKinds([]string{" replicaset ", " pod"})
	assert.True(t, result[model.KindReplicaSet])
	assert.True(t, result[model.KindPod])
}

func TestParseExcludeKinds_DuplicateAliases(t *testing.T) {
	result := parseExcludeKinds([]string{"deploy", "deployment"})
	assert.Len(t, result, 1)
	assert.True(t, result[model.KindDeployment])
}
