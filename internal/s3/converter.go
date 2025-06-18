package s3

import (
	"github.com/seike460/s3ry/pkg/types"
)

// ToTypesDownloadRequest converts s3.DownloadRequest to types.DownloadRequest
func ToTypesDownloadRequest(req DownloadRequest) types.DownloadRequest {
	return types.DownloadRequest{
		Bucket:   req.Bucket,
		Key:      req.Key,
		FilePath: req.FilePath,
	}
}

// ToTypesUploadRequest converts s3.UploadRequest to types.UploadRequest
func ToTypesUploadRequest(req UploadRequest) types.UploadRequest {
	return types.UploadRequest{
		Bucket:      req.Bucket,
		Key:         req.Key,
		FilePath:    req.FilePath,
		ContentType: req.ContentType,
		Metadata:    req.Metadata,
	}
}

// ToTypesListRequest converts s3.ListRequest to types.ListRequest
func ToTypesListRequest(req ListRequest) types.ListRequest {
	return types.ListRequest{
		Bucket:     req.Bucket,
		Prefix:     req.Prefix,
		Delimiter:  req.Delimiter,
		MaxKeys:    req.MaxKeys,
		StartAfter: req.StartAfter,
	}
}

// FromTypesObject converts types.Object to s3.Object
func FromTypesObject(obj types.Object) Object {
	return Object{
		Key:          obj.Key,
		Size:         obj.Size,
		LastModified: obj.LastModified,
		ETag:         obj.ETag,
		StorageClass: obj.StorageClass,
	}
}

// FromTypesObjects converts []types.Object to []s3.Object
func FromTypesObjects(objs []types.Object) []Object {
	result := make([]Object, len(objs))
	for i, obj := range objs {
		result[i] = FromTypesObject(obj)
	}
	return result
}

// ToTypesProgressCallback converts s3.ProgressCallback to types.ProgressCallback
func ToTypesProgressCallback(callback ProgressCallback) types.ProgressCallback {
	if callback == nil {
		return nil
	}
	return types.ProgressCallback(callback)
}
