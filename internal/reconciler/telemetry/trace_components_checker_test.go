package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestTraceComponentsCheck(t *testing.T) {
	healthyGatewayCond := metav1.Condition{Type: conditions.TypeGatewayHealthy, Status: metav1.ConditionTrue, Reason: conditions.ReasonGatewayReady}
	configGeneratedCond := metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionTrue, Reason: conditions.ReasonGatewayConfigured}
	runningCondition := metav1.Condition{Type: conditions.TypeRunning, Status: metav1.ConditionTrue, Reason: conditions.ReasonTraceGatewayDeploymentReady}

	tests := []struct {
		name                string
		pipelines           []telemetryv1alpha1.TracePipeline
		metricPipelines     []telemetryv1alpha1.MetricPipeline
		logPipelines        []telemetryv1alpha1.LogPipeline
		telemetryInDeletion bool
		expectedCondition   *metav1.Condition
	}{
		{
			name:                "should be healthy if no pipelines deployed",
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeTraceComponentsHealthy,
				Status:  "True",
				Reason:  "NoPipelineDeployed",
				Message: "No pipelines have been deployed",
			},
		},
		{
			name: "should be healthy if all pipelines running",
			pipelines: []telemetryv1alpha1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(configGeneratedCond).
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypePending,
						Status: metav1.ConditionFalse,
						Reason: conditions.ReasonTraceGatewayDeploymentNotReady,
					}).
					WithStatusCondition(runningCondition).
					Build(),
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(configGeneratedCond).
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypePending,
						Status: metav1.ConditionFalse,
						Reason: conditions.ReasonTraceGatewayDeploymentNotReady,
					}).
					WithStatusCondition(runningCondition).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeTraceComponentsHealthy,
				Status:  "True",
				Reason:  conditions.ReasonComponentsRunning,
				Message: "All trace components are running",
			},
		},
		{
			name: "should not be healthy if one pipeline refs missing secret",
			pipelines: []telemetryv1alpha1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(configGeneratedCond).
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypePending,
						Status: metav1.ConditionFalse,
						Reason: conditions.ReasonTraceGatewayDeploymentNotReady,
					}).
					WithStatusCondition(runningCondition).
					Build(),
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeConfigurationGenerated,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonReferencedSecretMissing,
						Message: "One or more referenced Secrets are missing",
					}).
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypePending,
						Status: metav1.ConditionTrue,
						Reason: conditions.ReasonReferencedSecretMissing,
					}).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeTraceComponentsHealthy,
				Status:  "False",
				Reason:  conditions.ReasonReferencedSecretMissing,
				Message: "One or more referenced Secrets are missing",
			},
		},
		{
			name: "should not be healthy if one pipeline waiting for gateway",
			pipelines: []telemetryv1alpha1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(configGeneratedCond).
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypePending,
						Status: metav1.ConditionFalse,
						Reason: conditions.ReasonTraceGatewayDeploymentNotReady,
					}).
					WithStatusCondition(runningCondition).
					Build(),
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeGatewayHealthy,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonGatewayNotReady,
						Message: conditions.MessageForTracePipeline(conditions.ReasonGatewayNotReady),
					}).
					WithStatusCondition(configGeneratedCond).
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypePending,
						Status: metav1.ConditionTrue,
						Reason: conditions.ReasonTraceGatewayDeploymentNotReady,
					}).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeTraceComponentsHealthy,
				Status:  "False",
				Reason:  "GatewayNotReady",
				Message: "Trace gateway Deployment is not ready",
			},
		},
		{
			name: "should not be healthy if max pipelines exceeded",
			pipelines: []telemetryv1alpha1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(configGeneratedCond).
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypePending,
						Status: metav1.ConditionFalse,
						Reason: conditions.ReasonTraceGatewayDeploymentNotReady,
					}).
					WithStatusCondition(runningCondition).
					Build(),
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeConfigurationGenerated,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonMaxPipelinesExceeded,
						Message: conditions.MessageForTracePipeline(conditions.ReasonMaxPipelinesExceeded),
					}).
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypePending,
						Status: metav1.ConditionTrue,
						Reason: conditions.ReasonMaxPipelinesExceeded,
					}).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeTraceComponentsHealthy,
				Status:  "False",
				Reason:  "MaxPipelinesExceeded",
				Message: "Maximum pipeline count limit exceeded",
			},
		},
		{
			name: "should prioritize ConfigGenerated reason over GatewayHealthy reason",
			pipelines: []telemetryv1alpha1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypeGatewayHealthy,
						Status: metav1.ConditionFalse,
						Reason: conditions.ReasonGatewayNotReady,
					}).
					WithStatusCondition(configGeneratedCond).
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypePending,
						Status: metav1.ConditionTrue,
						Reason: conditions.ReasonTraceGatewayDeploymentNotReady,
					}).
					Build(),
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeConfigurationGenerated,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonReferencedSecretMissing,
						Message: "One or more referenced Secrets are missing",
					}).
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypePending,
						Status: metav1.ConditionTrue,
						Reason: conditions.ReasonReferencedSecretMissing,
					}).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeTraceComponentsHealthy,
				Status:  "False",
				Reason:  conditions.ReasonReferencedSecretMissing,
				Message: "One or more referenced Secrets are missing",
			},
		},
		{
			name: "should block deletion if there are existing trace pipelines",
			pipelines: []telemetryv1alpha1.TracePipeline{
				testutils.NewTracePipelineBuilder().WithName("foo").Build(),
				testutils.NewTracePipelineBuilder().WithName("bar").Build(),
			},
			telemetryInDeletion: true,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeTraceComponentsHealthy,
				Status:  "False",
				Reason:  "ResourceBlocksDeletion",
				Message: "The deletion of the module is blocked. To unblock the deletion, delete the following resources: TracePipelines (bar,foo)",
			},
		},
		{
			name: "should not block deletion if there are existing metric pipelines",
			metricPipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("foo").Build(),
			},
			telemetryInDeletion: true,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeTraceComponentsHealthy,
				Status:  "True",
				Reason:  "NoPipelineDeployed",
				Message: "No pipelines have been deployed",
			},
		},
		{
			name: "should not block deletion if there are existing log pipelines",
			logPipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().WithName("foo").Build(),
			},
			telemetryInDeletion: true,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeTraceComponentsHealthy,
				Status:  "True",
				Reason:  "NoPipelineDeployed",
				Message: "No pipelines have been deployed",
			},
		},
		{
			name: "should be healthy if telemetry flow probing enabled and healthy",
			pipelines: []telemetryv1alpha1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypeFlowHealthy,
						Status: metav1.ConditionTrue,
						Reason: conditions.ReasonSelfMonFlowHealthy,
					}).
					Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeTraceComponentsHealthy,
				Status:  "True",
				Reason:  conditions.ReasonComponentsRunning,
				Message: "All trace components are running",
			},
		},
		{
			name: "should not be healthy if telemetry flow probing enabled and not healthy",
			pipelines: []telemetryv1alpha1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeFlowHealthy,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonSelfMonGatewayThrottling,
						Message: conditions.MessageForTracePipeline(conditions.ReasonSelfMonGatewayThrottling),
					}).
					Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeTraceComponentsHealthy,
				Status:  "False",
				Reason:  "GatewayThrottling",
				Message: "Trace gateway is unable to receive spans at current rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/03-traces?id=gateway-throttling",
			},
		},
		{
			name: "should show tlsCertExpert if one pipeline has invalid tls cert and the other pipeline has an about to expire cert",
			pipelines: []telemetryv1alpha1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(configGeneratedCond).
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypePending,
						Status: metav1.ConditionFalse,
						Reason: conditions.ReasonTraceGatewayDeploymentNotReady,
					}).
					WithStatusCondition(runningCondition).
					Build(),
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypeConfigurationGenerated,
						Status: metav1.ConditionTrue,
						Reason: conditions.ReasonTLSCertificateAboutToExpire,
					}).
					Build(),
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeConfigurationGenerated,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonTLSCertificateExpired,
						Message: "TLS certificate is expired",
					}).
					Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeTraceComponentsHealthy,
				Status:  "False",
				Reason:  "TLSCertificateExpired",
				Message: "TLS certificate is expired",
			},
		},
		{
			name: "should show tlsCert is about to expire if one of the pipelines has tls cert which is about to expire",
			pipelines: []telemetryv1alpha1.TracePipeline{
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(configGeneratedCond).
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypePending,
						Status: metav1.ConditionFalse,
						Reason: conditions.ReasonTraceGatewayDeploymentNotReady,
					}).
					WithStatusCondition(runningCondition).
					Build(),
				testutils.NewTracePipelineBuilder().
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeConfigurationGenerated,
						Status:  metav1.ConditionTrue,
						Reason:  conditions.ReasonTLSCertificateAboutToExpire,
						Message: "TLS certificate is about to expire",
					}).
					Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeTraceComponentsHealthy,
				Status:  "True",
				Reason:  "TLSCertificateAboutToExpire",
				Message: "TLS certificate is about to expire",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)

			b := fake.NewClientBuilder().WithScheme(scheme)
			for i := range test.pipelines {
				b.WithObjects(&test.pipelines[i])
			}
			fakeClient := b.Build()

			m := &traceComponentsChecker{
				client: fakeClient,
			}

			condition, err := m.Check(context.Background(), test.telemetryInDeletion)
			require.NoError(t, err)
			require.Equal(t, test.expectedCondition, condition)
		})
	}
}
