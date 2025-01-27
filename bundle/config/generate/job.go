package generate

import (
	"github.com/databricks/cli/libs/dyn"
	"github.com/databricks/cli/libs/dyn/yamlsaver"
	"github.com/databricks/databricks-sdk-go/service/jobs"
)

var jobOrder = yamlsaver.NewOrder([]string{"name", "job_clusters", "compute", "tasks"})
var taskOrder = yamlsaver.NewOrder([]string{"task_key", "depends_on", "existing_cluster_id", "new_cluster", "job_cluster_key"})

func ConvertJobToValue(job *jobs.Job) (dyn.Value, error) {
	value := make(map[string]dyn.Value)

	if job.Settings.Tasks != nil {
		tasks := make([]dyn.Value, 0)
		for _, task := range job.Settings.Tasks {
			v, err := convertTaskToValue(task, taskOrder)
			if err != nil {
				return dyn.InvalidValue, err
			}
			tasks = append(tasks, v)
		}
		// We're using location lines to define the order of keys in exported YAML.
		value["tasks"] = dyn.NewValue(tasks, dyn.Location{Line: jobOrder.Get("tasks")})
	}

	return yamlsaver.ConvertToMapValue(job.Settings, jobOrder, []string{"format", "new_cluster", "existing_cluster_id"}, value)
}

func convertTaskToValue(task jobs.Task, order *yamlsaver.Order) (dyn.Value, error) {
	dst := make(map[string]dyn.Value)
	return yamlsaver.ConvertToMapValue(task, order, []string{"format"}, dst)
}
