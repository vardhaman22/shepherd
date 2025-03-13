package cloudcredentials

// The json/yaml config key for the aws cloud credential config
const AlibabaCredentialConfigurationFileKey = "alibabaCredentials"

// AlibabaCloudCredentialConfig is configuration need to create an aws cloud credential
type AlibabaECSCredentialConfig struct {
	AccessKeyId     string `json:"accessKeyId" yaml:"accessKeyId"`
	AccessKeySecret string `json:"accessKeySecret" yaml:"accessKeySecret"`
	ApiEndpoint     string `json:"apiEndpoint,omitempty" yaml:"apiEndpoint"`
}
