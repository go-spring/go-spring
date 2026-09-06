/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package StarterS3

import ()

// Config defines the S3-protocol object storage client configuration.
//
// The starter speaks the S3 protocol through minio-go, so one config surface
// covers MinIO and AWS S3 natively, and the S3-compatible endpoints of other
// clouds (Aliyun OSS, Tencent COS, etc.); see README for the compatibility
// notes. Fields are intentionally flat, mirroring minio.Options.
type Config struct {
	// Endpoint is the S3 endpoint, host:port without scheme,
	// e.g., "127.0.0.1:9000" or "s3.amazonaws.com".
	Endpoint string `value:"${endpoint}" expr:"$ != ''"`

	// AccessKeyID is the access key of the static credential pair.
	AccessKeyID string `value:"${access-key-id}" expr:"$ != ''"`

	// SecretAccessKey is the secret key of the static credential pair.
	SecretAccessKey string `value:"${secret-access-key}" expr:"$ != ''"`

	// SessionToken is the optional session token (temporary credentials).
	SessionToken string `value:"${session-token:=}"`

	// Region is the bucket region, e.g., "us-east-1".
	Region string `value:"${region:=us-east-1}"`

	// UseSSL enables HTTPS towards the endpoint.
	UseSSL bool `value:"${use-ssl:=false}"`

	// BucketLookup selects how bucket names are addressed in URLs: "auto"
	// (the client decides), "virtual-host" (bucket.endpoint), "path"
	// (endpoint/bucket) or "dns". Some S3-compatible clouds only support the
	// path style.
	BucketLookup string `value:"${bucket-lookup:=auto}"`
}
