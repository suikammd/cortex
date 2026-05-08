package aliyun

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"

	"github.com/efficientgo/core/logerrcapture"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/encrypt"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/version"
	"github.com/thanos-io/objstore"
	thanoshttp "github.com/thanos-io/objstore/exthttp"

	cortexs3 "github.com/cortexproject/cortex/pkg/storage/bucket/s3"
)

type bucket struct {
	logger           log.Logger
	name             string
	client           *minio.Client
	defaultSSE       encrypt.ServerSide
	disableMultipart bool
	partSize         uint64
	listObjectsV1    bool
	sendContentMd5   bool
}

func newBucket(cfg Config, hedgedRoundTripper func(rt http.RoundTripper) http.RoundTripper, component string, logger log.Logger) (objstore.Bucket, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("no aliyun endpoint in config file")
	}

	provider, err := buildProvider(cfg)
	if err != nil {
		return nil, err
	}

	var tpt http.RoundTripper
	tpt, err = thanoshttp.DefaultTransport(thanoshttp.HTTPConfig{
		IdleConnTimeout:       model.Duration(cfg.HTTP.IdleConnTimeout),
		ResponseHeaderTimeout: model.Duration(cfg.HTTP.ResponseHeaderTimeout),
		InsecureSkipVerify:    cfg.HTTP.InsecureSkipVerify,
		TLSHandshakeTimeout:   model.Duration(cfg.HTTP.TLSHandshakeTimeout),
		ExpectContinueTimeout: model.Duration(cfg.HTTP.ExpectContinueTimeout),
		MaxIdleConns:          cfg.HTTP.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.HTTP.MaxIdleConnsPerHost,
		MaxConnsPerHost:       cfg.HTTP.MaxConnsPerHost,
		Transport:             cfg.HTTP.Transport,
	})
	if err != nil {
		return nil, err
	}
	if cfg.HTTP.Transport != nil {
		tpt = cfg.HTTP.Transport
	}
	if hedgedRoundTripper != nil {
		tpt = hedgedRoundTripper(tpt)
	}

	s3Cfg := cfg.baseS3Config()
	bucketLookupType := toMinioBucketLookupType(s3Cfg.BucketLookupType)

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:        credentials.NewChainCredentials([]credentials.Provider{provider}),
		Secure:       !cfg.Insecure,
		Region:       cfg.Region,
		Transport:    tpt,
		BucketLookup: bucketLookupType,
	})
	if err != nil {
		return nil, errors.Wrap(err, "initialize aliyun client")
	}
	client.SetAppInfo(fmt.Sprintf("cortex-%s", component), fmt.Sprintf("%s (%s)", version.Version, runtime.Version()))

	defaultSSE, err := cfg.SSE.BuildMinioConfig()
	if err != nil {
		return nil, err
	}

	if cfg.DisableDualstack {
		client.SetS3EnableDualstack(false)
	}

	return &bucket{
		logger:           logger,
		name:             cfg.BucketName,
		client:           client,
		defaultSSE:       defaultSSE,
		disableMultipart: false,
		partSize:         1024 * 1024 * 64,
		listObjectsV1:    cfg.ListObjectsVersion == cortexs3.ListObjectsVersionV1,
		sendContentMd5:   cfg.SendContentMd5,
	}, nil
}

func toMinioBucketLookupType(lookupType string) minio.BucketLookupType {
	switch lookupType {
	case cortexs3.BucketVirtualHostLookup:
		return minio.BucketLookupDNS
	case cortexs3.BucketPathLookup:
		return minio.BucketLookupPath
	default:
		return minio.BucketLookupAuto
	}
}

func (b *bucket) Provider() objstore.ObjProvider { return objstore.S3 }

func (b *bucket) Name() string { return b.name }

func (b *bucket) SupportedIterOptions() []objstore.IterOptionType {
	return []objstore.IterOptionType{objstore.Recursive, objstore.UpdatedAt}
}

func (b *bucket) IterWithAttributes(ctx context.Context, dir string, f func(attrs objstore.IterObjectAttributes) error, options ...objstore.IterOption) error {
	if err := objstore.ValidateIterOptions(b.SupportedIterOptions(), options...); err != nil {
		return err
	}
	if dir != "" {
		dir = strings.TrimSuffix(dir, "/") + "/"
	}

	appliedOpts := objstore.ApplyIterOptions(options...)
	opts := minio.ListObjectsOptions{
		Prefix:    dir,
		Recursive: appliedOpts.Recursive,
		UseV1:     b.listObjectsV1,
	}

	for object := range b.client.ListObjects(ctx, b.name, opts) {
		if object.Err != nil {
			return object.Err
		}
		if object.Key == "" || object.Key == dir {
			continue
		}

		attr := objstore.IterObjectAttributes{Name: object.Key}
		if appliedOpts.LastModified {
			attr.SetLastModified(object.LastModified)
		}
		if err := f(attr); err != nil {
			return err
		}
	}

	return ctx.Err()
}

func (b *bucket) Iter(ctx context.Context, dir string, f func(string) error, opts ...objstore.IterOption) error {
	var filteredOpts []objstore.IterOption
	for _, opt := range opts {
		if opt.Type == objstore.Recursive {
			filteredOpts = append(filteredOpts, opt)
			break
		}
	}

	return b.IterWithAttributes(ctx, dir, func(attrs objstore.IterObjectAttributes) error {
		return f(attrs.Name)
	}, filteredOpts...)
}

func (b *bucket) Get(ctx context.Context, name string) (io.ReadCloser, error) {
	return b.getRange(ctx, name, 0, -1)
}

func (b *bucket) GetRange(ctx context.Context, name string, off, length int64) (io.ReadCloser, error) {
	return b.getRange(ctx, name, off, length)
}

func (b *bucket) getRange(ctx context.Context, name string, off, length int64) (io.ReadCloser, error) {
	opts := &minio.GetObjectOptions{ServerSideEncryption: b.defaultSSE}
	if length != -1 {
		if err := opts.SetRange(off, off+length-1); err != nil {
			return nil, err
		}
	} else if off > 0 {
		if err := opts.SetRange(off, 0); err != nil {
			return nil, err
		}
	}

	r, err := b.client.GetObject(ctx, b.name, name, *opts)
	if err != nil {
		return nil, err
	}
	if _, err := r.Read(nil); err != nil {
		defer logerrcapture.Do(b.logger, r.Close, "aliyun get range obj close")
		return nil, err
	}

	return objstore.ObjectSizerReadCloser{
		ReadCloser: r,
		Size: func() (int64, error) {
			stat, err := r.Stat()
			if err != nil {
				return 0, err
			}
			return stat.Size, nil
		},
	}, nil
}

func (b *bucket) Exists(ctx context.Context, name string) (bool, error) {
	_, err := b.client.StatObject(ctx, b.name, name, minio.StatObjectOptions{
		ServerSideEncryption: b.defaultSSE,
	})
	if err != nil {
		if b.IsObjNotFoundErr(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "stat aliyun object")
	}

	return true, nil
}

func (b *bucket) Upload(ctx context.Context, name string, r io.Reader, opts ...objstore.ObjectUploadOption) error {
	size, err := objstore.TryToGetSize(r)
	if err != nil {
		level.Warn(b.logger).Log("msg", "could not guess file size for multipart upload; upload might be not optimized", "name", name, "err", err)
		size = -1
	}

	partSize := b.partSize
	if size < int64(partSize) {
		partSize = 0
	}

	uploadOpts := objstore.ApplyObjectUploadOptions(opts...)
	_, err = b.client.PutObject(ctx, b.name, name, r, size, minio.PutObjectOptions{
		DisableMultipart:     b.disableMultipart,
		PartSize:             partSize,
		ServerSideEncryption: b.defaultSSE,
		SendContentMd5:       b.sendContentMd5,
		NumThreads:           4,
		ContentType:          uploadOpts.ContentType,
	})
	if err != nil {
		return errors.Wrap(err, "upload aliyun object")
	}

	return nil
}

func (b *bucket) Attributes(ctx context.Context, name string) (objstore.ObjectAttributes, error) {
	objInfo, err := b.client.StatObject(ctx, b.name, name, minio.StatObjectOptions{
		ServerSideEncryption: b.defaultSSE,
	})
	if err != nil {
		return objstore.ObjectAttributes{}, err
	}

	return objstore.ObjectAttributes{
		Size:         objInfo.Size,
		LastModified: objInfo.LastModified,
	}, nil
}

func (b *bucket) Delete(ctx context.Context, name string) error {
	return b.client.RemoveObject(ctx, b.name, name, minio.RemoveObjectOptions{})
}

func (b *bucket) IsObjNotFoundErr(err error) bool {
	return minio.ToErrorResponse(errors.Cause(err)).Code == "NoSuchKey"
}

func (b *bucket) IsAccessDeniedErr(err error) bool {
	return minio.ToErrorResponse(errors.Cause(err)).Code == "AccessDenied"
}

func (b *bucket) Close() error { return nil }
