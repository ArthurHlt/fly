package commands

type FlyCommand struct {
	Target string `short:"t" long:"target" description:"Concourse target name or URL" default:"http://192.168.100.4:8080"`

	Login LoginCommand `command:"login" alias:"l" description:"Authenticate with the target"`
	Sync  SyncCommand  `command:"sync"  alias:"s" description:"Download and replace the current fly from the target"`

	Checklist ChecklistCommand `command:"checklist" alias:"cl" description:"Print a Checkfile of the given pipeline"`

	Execute ExecuteCommand `command:"execute" alias:"e" description:"Execute a one-off build using local bits"`
	Watch   WatchCommand   `command:"watch"   alias:"w" description:"Stream a build's output"`

	Containers ContainersCommand `command:"containers" alias:"cs" description:"Print the active containers"`
	Hijack     HijackCommand     `command:"hijack"     alias:"intercept" alias:"i" description:"Execute a command in a container"`

	Pipelines       PipelinesCommand       `command:"pipelines"        alias:"ps" description:"List the configured pipelines"`
	DestroyPipeline DestroyPipelineCommand `command:"destroy-pipeline" alias:"dp" description:"Destroy a pipeline"`
	GetPipeline     GetPipelineCommand     `command:"get-pipeline"     alias:"gp" description:"Get a pipeline's current configuration"`
	SetPipeline     SetPipelineCommand     `command:"set-pipeline"     alias:"sp" description:"Create or update a pipeline's configuration"`
	PausePipeline   PausePipelineCommand   `command:"pause-pipeline"   alias:"pp" description:"Pause a pipeline"`
	UnpausePipeline UnpausePipelineCommand `command:"unpause-pipeline" alias:"up" description:"Un-pause a pipeline"`

	Builds BuildsCommand `command:"builds" alias:"b" description:"List builds data"`

	TriggerJob TriggerJobCommand `command:"trigger-job"   alias:"t" description:"Trigger a job"`

	Volumes VolumesCommand `command:"volumes" alias:"vs" description:"List the active volumes"`
	Workers WorkersCommand `command:"workers" alias:"ws" description:"List the registered workers"`
}

var Fly FlyCommand
