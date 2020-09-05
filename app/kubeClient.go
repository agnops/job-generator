package main

import (
	"context"
	"flag"
	"fmt"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	coreV1Types "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	//"k8s.io/apimachinery/pkg/api/errors"
)

var clientset *kubernetes.Clientset
var secretsClient coreV1Types.SecretInterface
var configMapClient coreV1Types.ConfigMapInterface

var namespace = os.Getenv("NAMESPACE")
var cloudWrapperHostPort = os.Getenv("CLOUD_WRAPPER_HOST_PORT")

func getResourceList(cpu, memory string) apiv1.ResourceList {
	res := apiv1.ResourceList{}
	if cpu != "" {
		res[apiv1.ResourceCPU] = resource.MustParse(cpu)
	}
	if memory != "" {
		res[apiv1.ResourceMemory] = resource.MustParse(memory)
	}
	return res
}

func getJobName(scmWorkflowDetails *ScmWorkflowDetails, jobId int) string {
	filename := regexp.MustCompile(`[^a-zA-Z0-9-]`).ReplaceAllString(strings.Replace(strings.ToLower(scmWorkflowDetails.Workflow.FileName), ".yaml", "", -1), "-")
	gitOrgProject := regexp.MustCompile(`[^a-zA-Z0-9-]`).ReplaceAllString(strings.ToLower(scmWorkflowDetails.GitOrgProject), "-")
	gitRepository := regexp.MustCompile(`[^a-zA-Z0-9-]`).ReplaceAllString(strings.ToLower(scmWorkflowDetails.GitRepository), "-")
	branch := regexp.MustCompile(`[^a-zA-Z0-9-]`).ReplaceAllString(strings.ToLower(scmWorkflowDetails.Branch), "-")
	jobName := fmt.Sprintf("%s-%s-%s-%s-%s", gitOrgProject, gitRepository, branch, scmWorkflowDetails.CommitId[len(scmWorkflowDetails.CommitId)-7:], filename)
	if len(jobName) > 62 {
		jobName = jobName[0:62]
	}
	jobName = regexp.MustCompile(`[^0-9]$`).ReplaceAllString(jobName, "")
	jobName = jobName + strconv.Itoa(jobId)

	return jobName
}

func createJobObject(scmWorkflowDetails *ScmWorkflowDetails, jobId int) {
	jobName := getJobName(scmWorkflowDetails, jobId)

	sharedEnvs := []apiv1.EnvVar{
		{Name: "COMMITID", Value: scmWorkflowDetails.CommitId},
		{Name: "OAUTH_TOKEN", Value: scmWorkflowDetails.OAuthToken},
	}
	initContainer1Envs := []apiv1.EnvVar{
		{Name: "SCM_PROVIDER", Value: scmWorkflowDetails.ScProvider},
		{Name: "OAUTH_TOKEN", Value: scmWorkflowDetails.OAuthToken},
		{Name: "CLONEURL", Value: scmWorkflowDetails.CloneURL},
		{Name: "COMMITID", Value: scmWorkflowDetails.CommitId},
	}

	if len(scmWorkflowDetails.Workflow.WorkflowYaml.Workflow.GlobalAddOns.RepoName) > 0 {
		sharedEnvs = append(sharedEnvs, apiv1.EnvVar{Name: "REPO_NAME", Value: scmWorkflowDetails.Workflow.WorkflowYaml.Workflow.GlobalAddOns.RepoName})
		initContainer1Envs = append(initContainer1Envs, apiv1.EnvVar{Name: "REPO_NAME", Value: scmWorkflowDetails.Workflow.WorkflowYaml.Workflow.GlobalAddOns.RepoName})
	}
	if len(scmWorkflowDetails.Workflow.WorkflowYaml.Workflow.GlobalAddOns.DockerFilePath) > 0 {
		sharedEnvs = append(sharedEnvs, apiv1.EnvVar{Name: "DOCKERFILE_PATH", Value: scmWorkflowDetails.Workflow.WorkflowYaml.Workflow.GlobalAddOns.DockerFilePath})
	}

	dockerCloudOps := ""
	for _, dco := range scmWorkflowDetails.Workflow.WorkflowYaml.Workflow.GlobalAddOns.DockerCloudOps {
		dockerCloudOps += "_" + dco
	}
	if len(dockerCloudOps) > 0 {
		initContainer1Envs = append(initContainer1Envs, apiv1.EnvVar{Name: "DOCKER_CLOUDOPS", Value: dockerCloudOps})
		initContainer1Envs = append(initContainer1Envs, apiv1.EnvVar{Name: "CLOUD_WRAPPER_HOST_PORT", Value: cloudWrapperHostPort})
	}

	sharedEmptyDir := apiv1.EmptyDirVolumeSource{Medium: "", SizeLimit: nil}
	if len(scmWorkflowDetails.Workflow.WorkflowYaml.Workflow.GlobalAddOns.RAMDisk) > 0 {
		storageResources := apiv1.ResourceRequirements{}
		storageResources.Requests.Storage().Add(resource.MustParse(scmWorkflowDetails.Workflow.WorkflowYaml.Workflow.GlobalAddOns.RAMDisk))
		ramDiskSize := resource.MustParse(scmWorkflowDetails.Workflow.WorkflowYaml.Workflow.GlobalAddOns.RAMDisk)
		sharedEmptyDir = apiv1.EmptyDirVolumeSource{Medium: "Memory", SizeLimit: &ramDiskSize}
	}

	initContainers := []apiv1.Container{
		{
			Name:  "job-helper-init",
			Image: "agnops/job-helper",
			ImagePullPolicy: "Always",
			VolumeMounts: []apiv1.VolumeMount{{MountPath: "/data", Name: "containers-data"}},
			Env: initContainer1Envs,
		},
	}

	for i, container := range scmWorkflowDetails.Workflow.WorkflowYaml.Workflow.Containers {
		volumeMounts := []apiv1.VolumeMount{{MountPath: "/data", Name: "containers-data"}}
		var envFrom []apiv1.EnvFromSource
		args := ""

		if container.AddOns.IsDocker == true {
			volumeMounts = append(volumeMounts, []apiv1.VolumeMount{{MountPath: "/var/run/docker.sock", Name: "docker-sock"}, {MountPath: "/etc/docker/daemon.json", Name: "docker-daemon-json"}}...)
		}

		for _, envFromKey := range container.Kubernetes.EnvFrom {
			envFrom = append(envFrom, apiv1.EnvFromSource{SecretRef: &apiv1.SecretEnvSource{LocalObjectReference: apiv1.LocalObjectReference{Name: envFromKey.SecretRef.Name}}})
		}

		var resourcesRequests = getResourceList(container.Kubernetes.Resources.Requests.CPU, container.Kubernetes.Resources.Requests.Memory)
		var resourcesLimits = getResourceList(container.Kubernetes.Resources.Limits.CPU, container.Kubernetes.Resources.Limits.Memory)
		resources := apiv1.ResourceRequirements{Limits: resourcesLimits, Requests: resourcesRequests}

		for _, cmd := range strings.Split(container.Command, "\n") {
			if len(strings.TrimSpace(cmd)) > 0 {
				args += fmt.Sprintf("\n%s;", cmd)
			}
		}

		initContainers = append(initContainers, apiv1.Container{
			Name: 			 strconv.Itoa(i) + "-" + container.Name,
			Image:           container.Image,
			ImagePullPolicy: "Always",
			VolumeMounts:    volumeMounts,
			Env:             sharedEnvs,
			EnvFrom:         envFrom,
			Resources:       resources,
			Command:         []string{"sh", "-c"},
			Args:            append([]string{"cd /data/repo;" + args}),
		})
	}

	cdVolume := apiv1.Volume{Name: "containers-data", VolumeSource: apiv1.VolumeSource{EmptyDir: &sharedEmptyDir}}
	hostPathType := apiv1.HostPathType("File")
	dockerSockVolume := apiv1.Volume{Name: "docker-sock", VolumeSource: apiv1.VolumeSource{HostPath: &apiv1.HostPathVolumeSource{Path: "/var/run/docker.sock", Type: &hostPathType}}}
	dockerDaemonJsonVolume := apiv1.Volume{Name: "docker-daemon-json", VolumeSource: apiv1.VolumeSource{HostPath: &apiv1.HostPathVolumeSource{Path: "/etc/docker/daemon.json", Type: &hostPathType}}}
	backoffLimit := int32(0)
	ttlSecondsAfterFinished := int32(20)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"job_name": jobName, "agnops": "job"}},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "job-helper-finish",
							Image: "alpine",
							ImagePullPolicy: "Always",
							Env: []apiv1.EnvVar{
								{Name: "BRANCH", Value: scmWorkflowDetails.Branch},
								{Name: "COMMITMSG", Value: scmWorkflowDetails.CommitMsg},
								{Name: "COMMITURL", Value: scmWorkflowDetails.CommitUrl},
								{Name: "COMMITID", Value: scmWorkflowDetails.CommitId},
								{Name: "CLONEURL", Value: scmWorkflowDetails.CloneURL},
								{Name: "EMAIL", Value: scmWorkflowDetails.Email},
								{Name: "WORKFLOW_FILE_NAME", Value: scmWorkflowDetails.Workflow.FileName},
							},
						},
					},
					InitContainers: initContainers,
					RestartPolicy: "Never",
					NodeSelector: map[string]string{"nodegroup-type": "cicd-workloads"},
					Volumes: []apiv1.Volume{cdVolume, dockerSockVolume, dockerDaemonJsonVolume},
				},
			},
			BackoffLimit: &backoffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
		},
	}

	jobsClient := clientset.BatchV1().Jobs(namespace)
	log.Println("Creating job... ")
	result1, err := jobsClient.Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Println(err.Error())
		} else {
			failOnError(err, "Failed on job creation")
		}
	}
	log.Printf("Created job %q.\n", result1)
}

func createConfigMap(scmWorkflowDetails *ScmWorkflowDetails, jobId int) {

	configMapName := getJobName(scmWorkflowDetails, jobId)

	configMapSpec := apiv1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"AgnOps":      "invalidWorkflow",
			},
		},
		Data: map[string]string{
			"CloneURL":		scmWorkflowDetails.CloneURL,
			"Branch":		scmWorkflowDetails.Branch,
			"CommitMsg":	scmWorkflowDetails.CommitMsg,
			"CommitId":		scmWorkflowDetails.CommitId,
			"CommitUrl":	scmWorkflowDetails.CommitUrl,
			"Email":		scmWorkflowDetails.Email,
			"FileName":		scmWorkflowDetails.Workflow.FileName,
		},
	}

	_, err := configMapClient.Create(context.TODO(), &configMapSpec, metav1.CreateOptions{})
	if err != nil {
		log.Println(err.Error())
	}
	log.Printf("Created ConfigMap %s\n", configMapName)
}

func initK8sClientset() {
	var err error
	var config *rest.Config

	// use the current context in kubeconfig
	config, err = rest.InClusterConfig()

	if err != nil {
		if  strings.Contains(err.Error(), "unable to load in-cluster configuration") {
			var kubeconfig *string
			if home := homeDir(); home != "" {
				kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
			} else {
				kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
			}
			flag.Parse()
			config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		} else {
			log.Println(err.Error())
		}
	}
	// create the clientset
	clientset, err = kubernetes.NewForConfig(config)
	failOnError(err, "Failed on clientset init")
	secretsClient = clientset.CoreV1().Secrets(namespace)
	configMapClient = clientset.CoreV1().ConfigMaps(namespace)
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}