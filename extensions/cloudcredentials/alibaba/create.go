package alibaba

import (
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/cloudcredentials"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/defaults/namespaces"
	"github.com/rancher/shepherd/extensions/defaults/providers"
	"github.com/rancher/shepherd/extensions/defaults/stevetypes"
	"github.com/rancher/shepherd/extensions/steve"
	"github.com/rancher/shepherd/pkg/namegenerator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateAWSCloudCredentials is a helper function that creates V1 cloud credentials and waits for them to become active.
func CreateAlibabaCloudCredentials(client *rancher.Client, credentials cloudcredentials.CloudCredential) (*v1.SteveAPIObject, error) {
	secretName := namegenerator.AppendRandomString(providers.Alibaba)
	spec := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: cloudcredentials.GeneratedName,
			Namespace:    namespaces.CattleData,
			Annotations: map[string]string{
				"provisioning.cattle.io/driver": providers.Alibaba,
				"field.cattle.io/name":          secretName,
				"field.cattle.io/creatorId":     client.UserID,
			},
		},
		Data: map[string][]byte{
			"alibabacloudcredentialConfig-accessKeyId":     []byte(credentials.AlibabacloudCredentialConfig.AccessKeyId),
			"alibabacloudcredentialConfig-accessKeySecret": []byte(credentials.AlibabacloudCredentialConfig.AccessKeySecret),
			"alibabacloudcredentialConfig-apiEndpoint":     []byte(credentials.AlibabacloudCredentialConfig.ApiEndpoint),
		},
		Type: corev1.SecretTypeOpaque,
	}

	awsCloudCredentials, err := steve.CreateAndWaitForResource(client, stevetypes.Secret, spec, true, defaults.FiveSecondTimeout, defaults.FiveMinuteTimeout)
	if err != nil {
		return nil, err
	}

	return awsCloudCredentials, nil
}
