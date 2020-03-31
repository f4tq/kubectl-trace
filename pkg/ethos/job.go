package ethos

import (
	"fmt"
	"io"
	"io/ioutil"
	"strconv"

	"github.com/iovisor/kubectl-trace/pkg/meta"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	batchv1typed "k8s.io/client-go/kubernetes/typed/batch/v1"
	corev1typed "k8s.io/client-go/kubernetes/typed/core/v1"
)

type JobClient struct {
	JobClient    batchv1typed.JobInterface
	ConfigClient corev1typed.ConfigMapInterface
	outStream    io.Writer
}

// Job is a container of info needed to create the job responsible for tracing.
type Job struct {
	Name                string
	ID                  types.UID
	Namespace           string
	ServiceAccount      string
	Hostname            string
	Program             string
	PodUID              string
	ContainerName       string
	IsPod               bool
	ImageNameTag        string
	InitImageNameTag    string
	FetchHeaders        bool
	Deadline            int64
	DeadlineGracePeriod int64
	StartTime           *metav1.Time
	Status              JobStatus
}

// WithOutStream setup a file stream to output trace job operation information
func (t *JobClient) WithOutStream(o io.Writer) {
	if o == nil {
		t.outStream = ioutil.Discard
	}
	t.outStream = o
}

type JobFilter struct {
	Name *string
	ID   *types.UID
}

func (nf JobFilter) selectorOptions() metav1.ListOptions {
	selectorOptions := metav1.ListOptions{}

	if nf.Name != nil {
		selectorOptions = metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", meta.EthosLabelKey, *nf.Name),
		}
	}

	if nf.ID != nil {
		selectorOptions = metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", meta.EthosIDLabelKey, *nf.ID),
		}
	}

	if nf.Name == nil && nf.ID == nil {
		selectorOptions = metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s", meta.EthosIDLabelKey),
		}
	}

	return selectorOptions
}

func (t *JobClient) findJobsWithFilter(nf JobFilter) ([]batchv1.Job, error) {

	selectorOptions := nf.selectorOptions()
	if len(selectorOptions.LabelSelector) == 0 {
		return []batchv1.Job{}, nil
	}

	jl, err := t.JobClient.List(selectorOptions)

	if err != nil {
		return nil, err
	}
	return jl.Items, nil
}

func (t *JobClient) findConfigMapsWithFilter(nf JobFilter) ([]apiv1.ConfigMap, error) {
	selectorOptions := nf.selectorOptions()
	if len(selectorOptions.LabelSelector) == 0 {
		return []apiv1.ConfigMap{}, nil
	}

	cm, err := t.ConfigClient.List(selectorOptions)

	if err != nil {
		return nil, err
	}
	return cm.Items, nil
}

func (t *JobClient) GetJob(nf JobFilter) ([]Job, error) {
	jl, err := t.findJobsWithFilter(nf)
	if err != nil {
		return nil, err
	}
	tjobs := []Job{}

	for _, j := range jl {
		labels := j.GetLabels()
		name, ok := labels[meta.EthosLabelKey]
		if !ok {
			name = ""
		}
		id, ok := labels[meta.EthosIDLabelKey]
		if !ok {
			id = ""
		}
		hostname, err := jobHostname(j)
		if err != nil {
			hostname = ""
		}
		tj := Job{
			Name:      name,
			ID:        types.UID(id),
			Namespace: j.Namespace,
			Hostname:  hostname,
			StartTime: j.Status.StartTime,
			Status:    jobStatus(j),
		}
		tjobs = append(tjobs, tj)
	}

	return tjobs, nil
}

func (t *JobClient) DeleteJobs(nf JobFilter) error {
	nothingDeleted := true
	jl, err := t.findJobsWithFilter(nf)
	if err != nil {
		return err
	}

	dp := metav1.DeletePropagationForeground
	for _, j := range jl {
		err := t.JobClient.Delete(j.Name, &metav1.DeleteOptions{
			GracePeriodSeconds: int64Ptr(0),
			PropagationPolicy:  &dp,
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(t.outStream, "ethos job %s deleted\n", j.Name)
		nothingDeleted = false
	}

	cl, err := t.findConfigMapsWithFilter(nf)

	if err != nil {
		return err
	}

	for _, c := range cl {
		err := t.ConfigClient.Delete(c.Name, nil)
		if err != nil {
			return err
		}
		fmt.Fprintf(t.outStream, "ethos configuration %s deleted\n", c.Name)
		nothingDeleted = false
	}

	if nothingDeleted {
		fmt.Fprintf(t.outStream, "error: no ethos found to be deleted\n")
	}
	return nil
}

func (t *JobClient) CreateJob(nj Job) (*batchv1.Job, error) {

	bpfTraceCmd := []string{
		"/usr/bin/timeout",
		"--preserve-status",
		"--signal",
		"INT",
		strconv.FormatInt(nj.Deadline, 10),
		"/bin/trace-runner",
		"-b",
		"/programs/program",
	}

	if nj.IsPod {
		bpfTraceCmd = append(bpfTraceCmd, "--inpod")
		bpfTraceCmd = append(bpfTraceCmd, "--container="+nj.ContainerName)
		bpfTraceCmd = append(bpfTraceCmd, "--poduid="+nj.PodUID)
	}

	commonMeta := metav1.ObjectMeta{
		Name:      nj.Name,
		Namespace: nj.Namespace,
		Labels: map[string]string{
			meta.EthosLabelKey:   nj.Name,
			meta.EthosIDLabelKey: string(nj.ID),
		},
		Annotations: map[string]string{
			meta.EthosLabelKey:   nj.Name,
			meta.EthosIDLabelKey: string(nj.ID),
		},
	}

	cm := &apiv1.ConfigMap{
		ObjectMeta: commonMeta,
		Data: map[string]string{
			"program": nj.Program,
		},
	}

	job := &batchv1.Job{
		ObjectMeta: commonMeta,
		Spec: batchv1.JobSpec{
			ActiveDeadlineSeconds:   int64Ptr(nj.Deadline + nj.DeadlineGracePeriod),
			TTLSecondsAfterFinished: int32Ptr(5),
			Parallelism:             int32Ptr(1),
			Completions:             int32Ptr(1),
			BackoffLimit:            int32Ptr(1),
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: commonMeta,
				Spec: apiv1.PodSpec{
					HostPID:            true,
					ServiceAccountName: nj.ServiceAccount,
					Volumes: []apiv1.Volume{
						apiv1.Volume{
							Name: "program",
							VolumeSource: apiv1.VolumeSource{
								ConfigMap: &apiv1.ConfigMapVolumeSource{
									LocalObjectReference: apiv1.LocalObjectReference{
										Name: cm.Name,
									},
									DefaultMode: int32Ptr(0777),
								},
							},
						},
						apiv1.Volume{
							Name: "usr-host",
							VolumeSource: apiv1.VolumeSource{
								HostPath: &apiv1.HostPathVolumeSource{
									Path: "/usr",
								},
							},
						},
						apiv1.Volume{
							Name: "modules-host",
							VolumeSource: apiv1.VolumeSource{
								HostPath: &apiv1.HostPathVolumeSource{
									Path: "/lib/modules",
								},
							},
						},
						apiv1.Volume{
							Name: "sys",
							VolumeSource: apiv1.VolumeSource{
								HostPath: &apiv1.HostPathVolumeSource{
									Path: "/sys",
								},
							},
						},
					},
					Containers: []apiv1.Container{
						apiv1.Container{
							Name:    nj.Name,
							Image:   nj.ImageNameTag,
							Command: bpfTraceCmd,
							TTY:     true,
							Stdin:   true,
							Resources: apiv1.ResourceRequirements{
								Requests: apiv1.ResourceList{
									apiv1.ResourceCPU:    resource.MustParse("100m"),
									apiv1.ResourceMemory: resource.MustParse("100Mi"),
								},
								Limits: apiv1.ResourceList{
									apiv1.ResourceCPU:    resource.MustParse("1"),
									apiv1.ResourceMemory: resource.MustParse("1G"),
								},
							},
							VolumeMounts: []apiv1.VolumeMount{
								apiv1.VolumeMount{
									Name:      "program",
									MountPath: "/programs",
									ReadOnly:  true,
								},
								apiv1.VolumeMount{
									Name:      "sys",
									MountPath: "/sys",
									ReadOnly:  true,
								},
							},
							SecurityContext: &apiv1.SecurityContext{
								Privileged: boolPtr(true),
							},
							// We want to send SIGINT prior to the pod being killed, so we can print the map
							// we will also wait for an arbitrary amount of time (10s) to give bpftrace time to
							// process and summarize the data
							Lifecycle: &apiv1.Lifecycle{
								PreStop: &apiv1.Handler{
									Exec: &apiv1.ExecAction{
										Command: []string{
											"/bin/bash",
											"-c",
											fmt.Sprintf("kill -SIGINT $(pidof bpftrace) && sleep %s", strconv.FormatInt(nj.DeadlineGracePeriod, 10)),
										},
									},
								},
							},
						},
					},
					RestartPolicy: "Never",
					Affinity: &apiv1.Affinity{
						NodeAffinity: &apiv1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &apiv1.NodeSelector{
								NodeSelectorTerms: []apiv1.NodeSelectorTerm{
									apiv1.NodeSelectorTerm{
										MatchExpressions: []apiv1.NodeSelectorRequirement{
											apiv1.NodeSelectorRequirement{
												Key:      "kubernetes.io/hostname",
												Operator: apiv1.NodeSelectorOpIn,
												Values:   []string{nj.Hostname},
											},
										},
									},
								},
							},
						},
					},
					Tolerations: []apiv1.Toleration{
						apiv1.Toleration{
							Effect:   apiv1.TaintEffectNoSchedule,
							Operator: apiv1.TolerationOpExists,
						},
					},
				},
			},
		},
	}

	if nj.FetchHeaders {
		// If we aren't downloading headers, add the initContainer and set up mounts
		job.Spec.Template.Spec.InitContainers = []apiv1.Container{
			apiv1.Container{
				Name:  "kubectl-trace-init",
				Image: nj.InitImageNameTag,
				Resources: apiv1.ResourceRequirements{
					Requests: apiv1.ResourceList{
						apiv1.ResourceCPU:    resource.MustParse("100m"),
						apiv1.ResourceMemory: resource.MustParse("100Mi"),
					},
					Limits: apiv1.ResourceList{
						apiv1.ResourceCPU:    resource.MustParse("1"),
						apiv1.ResourceMemory: resource.MustParse("1G"),
					},
				},
				VolumeMounts: []apiv1.VolumeMount{
					apiv1.VolumeMount{
						Name:      "lsb-release",
						MountPath: "/etc/lsb-release.host",
						ReadOnly:  true,
					},
					apiv1.VolumeMount{
						Name:      "os-release",
						MountPath: "/etc/os-release.host",
						ReadOnly:  true,
					},
					apiv1.VolumeMount{
						Name:      "modules-dir",
						MountPath: "/lib/modules",
					},
					apiv1.VolumeMount{
						Name:      "modules-host",
						MountPath: "/lib/modules.host",
						ReadOnly:  true,
					},
					apiv1.VolumeMount{
						Name:      "linux-headers-generated",
						MountPath: "/usr/src/",
					},
				},
			},
		}

		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes,
			apiv1.Volume{
				Name: "lsb-release",
				VolumeSource: apiv1.VolumeSource{
					HostPath: &apiv1.HostPathVolumeSource{
						Path: "/etc/lsb-release",
					},
				},
			},
			apiv1.Volume{
				Name: "os-release",
				VolumeSource: apiv1.VolumeSource{
					HostPath: &apiv1.HostPathVolumeSource{
						Path: "/etc/os-release",
					},
				},
			},
			apiv1.Volume{
				Name: "modules-dir",
				VolumeSource: apiv1.VolumeSource{
					HostPath: &apiv1.HostPathVolumeSource{
						Path: "/var/cache/linux-headers/modules_dir",
					},
				},
			},
			apiv1.Volume{
				Name: "linux-headers-generated",
				VolumeSource: apiv1.VolumeSource{
					HostPath: &apiv1.HostPathVolumeSource{
						Path: "/var/cache/linux-headers/generated",
					},
				},
			})

		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts,
			apiv1.VolumeMount{
				Name:      "modules-dir",
				MountPath: "/lib/modules",
				ReadOnly:  true,
			},
			apiv1.VolumeMount{
				Name:      "modules-host",
				MountPath: "/lib/modules.host",
				ReadOnly:  true,
			},
			apiv1.VolumeMount{
				Name:      "linux-headers-generated",
				MountPath: "/usr/src/kernels",
				ReadOnly:  true,
			})

	} else {
		// If we aren't downloading headers, unconditionally used the ones linked in /lib/modules
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts,
			apiv1.VolumeMount{
				Name:      "usr-host",
				MountPath: "/usr-host",
				ReadOnly:  true,
			},
			apiv1.VolumeMount{
				Name:      "modules-host",
				MountPath: "/lib/modules",
				ReadOnly:  true,
			})
	}
	if _, err := t.ConfigClient.Create(cm); err != nil {
		return nil, err
	}
	return t.JobClient.Create(job)
}

func int32Ptr(i int32) *int32 { return &i }
func int64Ptr(i int64) *int64 { return &i }
func boolPtr(b bool) *bool    { return &b }

func jobHostname(j batchv1.Job) (string, error) {
	aff := j.Spec.Template.Spec.Affinity
	if aff == nil {
		return "", fmt.Errorf("affinity not found for job")
	}

	nodeAff := aff.NodeAffinity

	if nodeAff == nil {
		return "", fmt.Errorf("node affinity not found for job")
	}

	requiredScheduling := nodeAff.RequiredDuringSchedulingIgnoredDuringExecution

	if requiredScheduling == nil {
		return "", fmt.Errorf("node affinity RequiredDuringSchedulingIgnoredDuringExecution not found for job")
	}
	nst := requiredScheduling.NodeSelectorTerms
	if len(nst) == 0 {
		return "", fmt.Errorf("node selector terms are empty in node affinity for job")
	}

	me := nst[0].MatchExpressions

	if len(me) == 0 {
		return "", fmt.Errorf("node selector terms match expressions are empty in node affinity for job")
	}

	for _, v := range me {
		if v.Key == "kubernetes.io/hostname" {
			if len(v.Values) == 0 {
				return "", fmt.Errorf("hostname affinity found but no values in it for job")
			}

			return v.Values[0], nil
		}
	}

	return "", fmt.Errorf("hostname not found for job")
}

// JobStatus is a label for the running status of a trace job at the current time.
type JobStatus string

// These are the valid status of traces.
const (
	// JobRunning means the trace job has active pods.
	JobRunning JobStatus = "Running"
	// JobCompleted means the trace job does not have any active pod and has success pods.
	JobCompleted JobStatus = "Completed"
	// JobFailed means the trace job does not have any active or success pod and has fpods that failed.
	JobFailed JobStatus = "Failed"
	// JobUnknown means that for some reason we do not have the information to determine the status.
	JobUnknown JobStatus = "Unknown"
)

func jobStatus(j batchv1.Job) JobStatus {
	if j.Status.Active > 0 {
		return JobRunning
	}
	if j.Status.Succeeded > 0 {
		return JobCompleted
	}
	if j.Status.Failed > 0 {
		return JobFailed
	}
	return JobUnknown
}
