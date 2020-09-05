module github.com/agnops/job-generator

go 1.13

require (
	github.com/go-git/go-git/v5 v5.0.0
	github.com/golang/protobuf v1.4.2
	github.com/gorilla/handlers v1.4.2
	github.com/micro/go-micro v1.18.0
	github.com/onsi/ginkgo v1.13.0 // indirect
	github.com/streadway/amqp v0.0.0-20200108173154-1c71cc93ed71
	gitlab.com/gitlab-org/project-templates/go-micro v0.0.0-20190225134054-1385fab987bc
	gopkg.in/go-playground/webhooks.v5 v5.14.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	k8s.io/api v0.0.0-20200726131424-9540e4cac147
	k8s.io/apimachinery v0.0.0-20200726131235-945d4ebf362b
	k8s.io/client-go v0.0.0-20200726131703-36233866f1c7
	k8s.io/klog v1.0.0 // indirect
	k8s.io/klog/v2 v2.2.0
	k8s.io/utils v0.0.0-20200720150651-0bdb4ca86cbc
	sigs.k8s.io/yaml v1.2.0
)

replace (
	k8s.io/api => k8s.io/api v0.0.0-20200726131424-9540e4cac147
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20200726131235-945d4ebf362b
)
