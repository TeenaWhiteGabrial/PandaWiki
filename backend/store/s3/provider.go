package s3

import "github.com/google/wire"

var ProviderSet = wire.NewSet(NewMinioClient, NewImageOSSClient)
