package imagesystem

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/acorn-io/acorn/pkg/config"
	"github.com/acorn-io/acorn/pkg/system"
	"github.com/google/go-containerregistry/pkg/name"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func RegistryExists(ctx context.Context, c client.Reader) (bool, error) {
	dep := &appsv1.Deployment{}
	err := c.Get(ctx, client.ObjectKey{
		Name:      system.RegistryName,
		Namespace: system.ImagesNamespace,
	}, dep)
	if apierrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

func GetInternalRepoForNamespace(ctx context.Context, c client.Reader, namespace string) (name.Repository, error) {
	cfg, err := config.Get(ctx, c)
	if err != nil {
		return name.Repository{}, err
	}
	if cfg.InternalRegistryPrefix != "" {
		return name.NewRepository(cfg.InternalRegistryPrefix + namespace)
	}

	dns, err := GetClusterInternalRegistryDNSName(ctx, c)
	if err != nil {
		return name.Repository{}, err
	}

	return name.NewRepository(fmt.Sprintf("%s:%d/acorn/%s", dns, system.RegistryPort, namespace))
}

func GetRuntimePullableInternalRepoForNamespace(ctx context.Context, c client.Reader, namespace string) (name.Repository, error) {
	cfg, err := config.Get(ctx, c)
	if err != nil {
		return name.Repository{}, err
	}
	if cfg.InternalRegistryPrefix != "" {
		return name.NewRepository(cfg.InternalRegistryPrefix + namespace)
	}

	address, err := GetClusterInternalRegistryAddress(ctx, c)
	if err != nil {
		return name.Repository{}, err
	}

	return name.NewRepository(fmt.Sprintf("%s/acorn/%s", address, namespace))
}

func GetRuntimePullableInternalRepoForNamespaceAndID(ctx context.Context, c client.Reader, namespace, imageID string) (name.Reference, error) {
	repo, err := GetRuntimePullableInternalRepoForNamespace(ctx, c, namespace)
	if err != nil {
		return nil, err
	}
	return repo.Digest("sha256:" + imageID), nil
}

func GetInternalRepoForNamespaceAndID(ctx context.Context, c client.Reader, namespace, imageID string) (name.Reference, error) {
	repo, err := GetInternalRepoForNamespace(ctx, c, namespace)
	if err != nil {
		return nil, err
	}
	return repo.Digest("sha256:" + imageID), nil
}

func GetRegistryObjects(ctx context.Context, c client.Reader) (result []client.Object, _ error) {
	cfg, err := config.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	if cfg.InternalRegistryPrefix != "" {
		return nil, nil
	}

	result = append(result, registryService(system.ImagesNamespace)...)

	// we won't be able to find this service at first, so ignore the 404s
	port, err := getRegistryPort(ctx, c)
	if err == nil {
		result = append(result, containerdConfigPathDaemonSet(system.ImagesNamespace, system.DefaultImage(), strconv.Itoa(port))...)
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	result = append(result, registryDeployment(system.ImagesNamespace, system.DefaultImage())...)
	return result, nil
}

func GetClusterInternalRegistryDNSName(ctx context.Context, c client.Reader) (string, error) {
	cfg, err := config.Get(ctx, c)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s.%s", system.RegistryName, system.ImagesNamespace, cfg.InternalClusterDomain), err
}

func GetClusterInternalRegistryAddress(ctx context.Context, c client.Reader) (string, error) {
	port, err := getRegistryPort(ctx, c)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("127.0.0.1:%d", port), nil
}

func getRegistryPort(ctx context.Context, c client.Reader) (int, error) {
	var service corev1.Service
	err := c.Get(ctx, client.ObjectKey{Name: system.RegistryName, Namespace: system.ImagesNamespace}, &service)
	if err != nil {
		return 0, fmt.Errorf("getting %s/%s service: %w", system.Namespace, system.RegistryName, err)
	}
	for _, port := range service.Spec.Ports {
		if port.Name == system.RegistryName && port.NodePort > 0 {
			return int(port.NodePort), nil
		}
	}

	return 0, fmt.Errorf("failed to find node port for registry %s/%s", system.Namespace, system.RegistryName)
}

func ParseAndEnsureNotInternalRepo(ctx context.Context, c client.Reader, image string) (name.Reference, error) {
	if os.Getenv("ACORN_TEST_ALLOW_LOCALHOST_REGISTRY") != "true" && strings.HasPrefix(image, "127.") {
		return nil, fmt.Errorf("invalid image reference %s", image)
	}
	cfg, err := config.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	if cfg.InternalRegistryPrefix != "" {
		if strings.HasPrefix(image, cfg.InternalRegistryPrefix) {
			return nil, fmt.Errorf("invalid image reference prefix %s", image)
		}
	}
	return name.ParseReference(image)
}
