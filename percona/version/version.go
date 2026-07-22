package version

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/percona/percona-postgresql-operator/v2/percona/version/service/client"
	"github.com/percona/percona-postgresql-operator/v2/percona/version/service/client/version_service"
)

//go:generate sh -c "yq -i '.metadata.labels.\"pgv2.percona.com/version\" = \"v\" + load(\"version.txt\")' ../../config/crd/patches/versionlabel_in_perconapgclusters.yaml"
//go:generate sh -c "yq -i '.metadata.labels.\"pgv2.percona.com/version\" = \"v\" + load(\"version.txt\")' ../../config/crd/patches/versionlabel_in_perconapgbackups.yaml"
//go:generate sh -c "yq -i '.metadata.labels.\"pgv2.percona.com/version\" = \"v\" + load(\"version.txt\")' ../../config/crd/patches/versionlabel_in_perconapgrestores.yaml"
//go:generate sh -c "yq -i '.metadata.labels.\"pgv2.percona.com/version\" = \"v\" + load(\"version.txt\")' ../../config/crd/patches/versionlabel_in_perconapgupgrade.yaml"

//go:embed version.txt
var version string

func Version() string {
	return strings.TrimSpace(version)
}

// ComparableVersion returns Version with any build suffix (e.g. "-lineaje-01")
// stripped, so it can be safely passed to semver comparisons such as
// CompareVersion. Semver treats a "-suffix" as a pre-release, which sorts
// lower than the plain release, so passing Version() directly there would
// make current-version clusters compare as older than they are.
func ComparableVersion() string {
	v := Version()
	if idx := strings.Index(v, "-"); idx != -1 {
		return v[:idx]
	}
	return v
}

const ProductName = "pg-operator"

type Meta struct {
	Apply              string
	OperatorVersion    string
	PGVersion          string
	KubeVersion        string
	Platform           string
	PMMVersion         string
	BackupVersion      string
	CRUID              string
	PMMEnabled         bool
	HelmDeployCR       bool
	HelmDeployOperator bool
	SidecarsUsed       bool
	Extensions         string
}

const DefaultVersionServiceEndpoint = "https://check.percona.com"

func getDefaultVersionServiceEndpoint() string {
	endpoint := os.Getenv("PERCONA_VS_FALLBACK_URI")

	if len(endpoint) != 0 {
		return endpoint
	}

	return DefaultVersionServiceEndpoint
}

func EnsureVersion(ctx context.Context, meta Meta) error {
	err := fetchVersions(ctx, getDefaultVersionServiceEndpoint(), meta)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to send telemetry to %s", getDefaultVersionServiceEndpoint()))
	}

	return nil
}

func fetchVersions(ctx context.Context, endpoint string, vm Meta) error {
	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return errors.Wrap(err, "parse endpoint")
	}

	srvCl := client.NewHTTPClientWithConfig(nil, &client.TransportConfig{
		Host:     requestURL.Host,
		BasePath: requestURL.Path,
		Schemes:  []string{requestURL.Scheme},
	})

	applyParams := &version_service.VersionServiceApplyParams{
		Context:            ctx,
		HTTPClient:         &http.Client{Timeout: 10 * time.Second},
		Product:            ProductName,
		Apply:              vm.Apply,
		BackupVersion:      &vm.BackupVersion,
		CustomResourceUID:  &vm.CRUID,
		DatabaseVersion:    &vm.PGVersion,
		KubeVersion:        &vm.KubeVersion,
		NamespaceUID:       new(string),
		OperatorVersion:    vm.OperatorVersion,
		Platform:           &vm.Platform,
		PmmVersion:         &vm.PMMVersion,
		PmmEnabled:         &vm.PMMEnabled,
		HelmDeployCr:       &vm.HelmDeployCR,
		HelmDeployOperator: &vm.HelmDeployOperator,
		SidecarsUsed:       &vm.SidecarsUsed,
		Extensions:         &vm.Extensions,
	}
	applyParams = applyParams.WithTimeout(10 * time.Second)

	_, err = srvCl.VersionService.VersionServiceApply(applyParams)
	if err != nil {
		return errors.Wrap(err, "version service apply")
	}

	return nil
}
