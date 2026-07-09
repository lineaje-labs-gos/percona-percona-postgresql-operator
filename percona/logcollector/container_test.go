package logcollector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/percona/percona-postgresql-operator/v2/internal/naming"
	"github.com/percona/percona-postgresql-operator/v2/internal/pgbackrest"
	"github.com/percona/percona-postgresql-operator/v2/internal/postgres"
	"github.com/percona/percona-postgresql-operator/v2/percona/logcollector/logrotate"
	"github.com/percona/percona-postgresql-operator/v2/percona/version"
	v2 "github.com/percona/percona-postgresql-operator/v2/pkg/apis/pgv2.percona.com/v2"
)

type builderFn func(cr *v2.PerconaPGCluster) ([]corev1.Container, error)

func TestContainers(t *testing.T) {
	testEnvVar := []corev1.EnvVar{
		{Name: "TEST_ENV1", Value: "test-value1"},
	}
	testEnvFrom := []corev1.EnvFromSource{
		{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "test-configmap",
				},
			},
		},
	}
	testLogRotate := &v2.LogRotateSpec{
		Configuration: "my-logrotate-config",
		ExtraConfig: corev1.LocalObjectReference{
			Name: "my-logrotate-extra-config",
		},
		Schedule: "0 0 * * *",
	}

	dataMount := postgres.DataVolumeMount()
	repoMount := pgbackrest.RepoVolumeMount()

	instanceLogEnv := []corev1.EnvVar{
		{Name: "PG_LOG_DIR", Value: postgresLogPath},
		{Name: "PGBACKREST_LOG_DIR", Value: naming.PGBackRestPGDataLogPath},
	}
	repoHostLogEnv := []corev1.EnvVar{
		{Name: "PG_LOG_DIR", Value: postgresLogPath},
		{Name: "PGBACKREST_LOG_DIR", Value: repoHostLogGlob},
	}

	tests := map[string]struct {
		builder                builderFn
		dataMount              corev1.VolumeMount
		logSpecificEnv         []corev1.EnvVar
		logCollector           *v2.LogCollectorSpec
		logRotate              *v2.LogRotateSpec
		expectedContainerNames []string
		build                  bool
	}{
		"nil logcollector (instance)": {
			builder:      instanceContainers,
			logCollector: nil,
		},
		"logcollector disabled (instance)": {
			builder: instanceContainers,
			logCollector: &v2.LogCollectorSpec{
				Enabled: false,
			},
		},
		"logcollector enabled (instance)": {
			builder:        instanceContainers,
			dataMount:      dataMount,
			logSpecificEnv: instanceLogEnv,
			logCollector: &v2.LogCollectorSpec{
				Enabled:         true,
				Image:           "log-test-image",
				ImagePullPolicy: corev1.PullIfNotPresent,
			},
			expectedContainerNames: []string{"logs", "logrotate"},
			build:                  true,
		},
		"logcollector enabled with configuration (instance)": {
			builder:        instanceContainers,
			dataMount:      dataMount,
			logSpecificEnv: instanceLogEnv,
			logCollector: &v2.LogCollectorSpec{
				Enabled:         true,
				Image:           "log-test-image",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Configuration:   "my-config",
			},
			expectedContainerNames: []string{"logs", "logrotate"},
			build:                  true,
		},
		"logcollector enabled with env variable (instance)": {
			builder:        instanceContainers,
			dataMount:      dataMount,
			logSpecificEnv: instanceLogEnv,
			logCollector: &v2.LogCollectorSpec{
				Enabled:         true,
				Image:           "log-test-image",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Configuration:   "my-config",
				Env:             testEnvVar,
				EnvFrom:         testEnvFrom,
			},
			expectedContainerNames: []string{"logs", "logrotate"},
			build:                  true,
		},
		"logcollector enabled with logrotate (instance)": {
			builder:        instanceContainers,
			dataMount:      dataMount,
			logSpecificEnv: instanceLogEnv,
			logCollector: &v2.LogCollectorSpec{
				Enabled:         true,
				Image:           "log-test-image",
				ImagePullPolicy: corev1.PullIfNotPresent,
			},
			logRotate:              testLogRotate,
			expectedContainerNames: []string{"logs", "logrotate"},
			build:                  true,
		},
		"logcollector enabled (repo-host)": {
			builder:        repoHostContainers,
			dataMount:      repoMount,
			logSpecificEnv: repoHostLogEnv,
			logCollector: &v2.LogCollectorSpec{
				Enabled:         true,
				Image:           "log-test-image",
				ImagePullPolicy: corev1.PullIfNotPresent,
			},
			expectedContainerNames: []string{"logs", "logrotate"},
			build:                  true,
		},
		"logcollector enabled with logrotate (repo-host)": {
			builder:        repoHostContainers,
			dataMount:      repoMount,
			logSpecificEnv: repoHostLogEnv,
			logCollector: &v2.LogCollectorSpec{
				Enabled:         true,
				Image:           "log-test-image",
				ImagePullPolicy: corev1.PullIfNotPresent,
			},
			logRotate:              testLogRotate,
			expectedContainerNames: []string{"logs", "logrotate"},
			build:                  true,
		},
	}

	for name, tt := range tests {
		spec := tt.logCollector
		if tt.logRotate != nil {
			spec.LogRotate = tt.logRotate
		}
		t.Run(name, func(t *testing.T) {
			cr := &v2.PerconaPGCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cluster",
					Namespace: "default",
				},
				Spec: v2.PerconaPGClusterSpec{
					CRVersion:    version.Version(),
					LogCollector: spec,
				},
			}

			containers, err := tt.builder(cr)
			assert.NoError(t, err)

			if !tt.build {
				assert.Nil(t, containers)
				return
			}

			var gotNames []string
			for _, c := range containers {
				gotNames = append(gotNames, c.Name)
			}
			assert.Equal(t, tt.expectedContainerNames, gotNames)

			expected := expectedContainers(tt.dataMount, tt.logSpecificEnv, spec.Configuration, spec.Env, spec.EnvFrom, tt.logRotate)
			assert.Equal(t, expected, containers)
		})
	}
}

func expectedContainers(
	dataMount corev1.VolumeMount,
	logSpecificEnv []corev1.EnvVar,
	configuration string,
	extraEnv []corev1.EnvVar,
	envFrom []corev1.EnvFromSource,
	logrotateConfig *v2.LogRotateSpec,
) []corev1.Container {
	envs := append([]corev1.EnvVar(nil), logSpecificEnv...)
	envs = append(envs,
		corev1.EnvVar{
			Name: "POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		corev1.EnvVar{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
	)
	envs = append(envs, extraEnv...)

	logsC := corev1.Container{
		Name:            "logs",
		Image:           "log-test-image",
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/opt/percona/logcollector/entrypoint.sh"},
		Args:            []string{"fluent-bit"},
		Env:             envs,
		EnvFrom:         envFrom,
		VolumeMounts: []corev1.VolumeMount{
			{Name: dataMount.Name, MountPath: dataMount.MountPath},
		},
	}
	if configuration != "" {
		logsC.VolumeMounts = append(logsC.VolumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: "/opt/percona/logcollector/fluentbit/custom",
		})
	}

	logRotateC := corev1.Container{
		Name:            "logrotate",
		Image:           "log-test-image",
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/opt/percona/logcollector/entrypoint.sh"},
		Args:            []string{"logrotate"},
		Env: []corev1.EnvVar{
			{
				Name: "POD_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name:  "LOGROTATE_SCHEDULE",
				Value: logrotateSchedule(logrotateConfig),
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: dataMount.Name, MountPath: dataMount.MountPath},
		},
	}
	if logrotateConfig != nil {
		if logrotateConfig.Configuration != "" || logrotateConfig.ExtraConfig.Name != "" {
			logRotateC.VolumeMounts = append(logRotateC.VolumeMounts, corev1.VolumeMount{
				Name:      logrotate.VolumeName,
				MountPath: "/opt/percona/logcollector/logrotate/conf.d",
			})
		}
	}

	return []corev1.Container{logsC, logRotateC}
}

func logrotateSchedule(lr *v2.LogRotateSpec) string {
	if lr != nil && lr.Schedule != "" {
		return lr.Schedule
	}
	return logrotate.DefaultSchedule
}

func TestVolumes(t *testing.T) {
	base := func(spec *v2.LogCollectorSpec) *v2.PerconaPGCluster {
		return &v2.PerconaPGCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "default"},
			Spec: v2.PerconaPGClusterSpec{
				CRVersion:    version.Version(),
				LogCollector: spec,
			},
		}
	}

	fluentBitVol := corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "my-cluster-log-collector-config"},
			},
		},
	}
	managedLogrotateSource := corev1.VolumeProjection{
		ConfigMap: &corev1.ConfigMapProjection{
			LocalObjectReference: corev1.LocalObjectReference{Name: "my-cluster-log-collector-logrotate-config"},
		},
	}
	extraLogrotateSource := corev1.VolumeProjection{
		ConfigMap: &corev1.ConfigMapProjection{
			LocalObjectReference: corev1.LocalObjectReference{Name: "extra-cm"},
		},
	}
	logrotateVol := func(sources ...corev1.VolumeProjection) corev1.Volume {
		return corev1.Volume{
			Name: logrotate.VolumeName,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{Sources: sources},
			},
		}
	}

	tests := map[string]struct {
		spec     *v2.LogCollectorSpec
		expected []corev1.Volume
	}{
		"disabled": {
			spec: &v2.LogCollectorSpec{Enabled: false, Configuration: "x"},
		},
		"nil": {},
		"enabled without configuration": {
			spec: &v2.LogCollectorSpec{Enabled: true},
		},
		"fluent-bit configuration only": {
			spec:     &v2.LogCollectorSpec{Enabled: true, Configuration: "cfg"},
			expected: []corev1.Volume{fluentBitVol},
		},
		"logrotate configuration only": {
			spec: &v2.LogCollectorSpec{
				Enabled:   true,
				LogRotate: &v2.LogRotateSpec{Configuration: "cfg"},
			},
			expected: []corev1.Volume{logrotateVol(managedLogrotateSource)},
		},
		"logrotate extraConfig only": {
			spec: &v2.LogCollectorSpec{
				Enabled:   true,
				LogRotate: &v2.LogRotateSpec{ExtraConfig: corev1.LocalObjectReference{Name: "extra-cm"}},
			},
			expected: []corev1.Volume{logrotateVol(extraLogrotateSource)},
		},
		"logrotate configuration and extraConfig": {
			spec: &v2.LogCollectorSpec{
				Enabled: true,
				LogRotate: &v2.LogRotateSpec{
					Configuration: "cfg",
					ExtraConfig:   corev1.LocalObjectReference{Name: "extra-cm"},
				},
			},
			expected: []corev1.Volume{logrotateVol(managedLogrotateSource, extraLogrotateSource)},
		},
		"all sources": {
			spec: &v2.LogCollectorSpec{
				Enabled:       true,
				Configuration: "fbcfg",
				LogRotate: &v2.LogRotateSpec{
					Configuration: "lrcfg",
					ExtraConfig:   corev1.LocalObjectReference{Name: "extra-cm"},
				},
			},
			expected: []corev1.Volume{fluentBitVol, logrotateVol(managedLogrotateSource, extraLogrotateSource)},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.expected, volumes(base(tt.spec)))
		})
	}
}
