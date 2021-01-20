package action

import (
	"context"
	"io"
	"os"

	imagesv1 "github.com/rancher/k3c/pkg/apis/services/images/v1alpha1"
	"github.com/rancher/k3c/pkg/client"
	"github.com/rancher/k3c/pkg/progress"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	criv1 "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/kubernetes/pkg/credentialprovider"
)

type PullImage struct {
	Force    bool   `usage:""`
	Platform string `usage:"Set platform if server is multi-platform capable"`
}

func (s *PullImage) Invoke(ctx context.Context, k8s *client.Interface, image string) error {
	return DoImages(ctx, k8s, func(ctx context.Context, imagesClient imagesv1.ImagesClient) error {
		eg, ctx := errgroup.WithContext(ctx)
		ch := make(chan []imagesv1.ImageStatus)
		eg.Go(func() error {
			return progress.Display(ch, os.Stdout)
		})
		eg.Go(func() error {
			defer close(ch)
			ppc, err := imagesClient.PullProgress(ctx, &imagesv1.ImageProgressRequest{Image: image})
			if err != nil {
				return err
			}
			for {
				info, err := ppc.Recv()
				if err == io.EOF {
					return nil
				}
				if err != nil {
					return err
				}
				ch <- info.Status
			}
			return nil
		})
		req := &imagesv1.ImagePullRequest{
			Image: &criv1.ImageSpec{
				Image: image,
			},
		}
		provider := credentialprovider.NewDockerKeyring()
		if auth, ok := provider.Lookup(image); ok {
			req.Auth = &criv1.AuthConfig{
				Username:      auth[0].Username,
				Password:      auth[0].Password,
				Auth:          auth[0].Auth,
				ServerAddress: auth[0].ServerAddress,
				IdentityToken: auth[0].IdentityToken,
				RegistryToken: auth[0].RegistryToken,
			}
		}
		res, err := imagesClient.Pull(ctx, req)
		if err != nil {
			return err
		}
		logrus.Debugf("%#v", res)
		return eg.Wait()
	})
}
