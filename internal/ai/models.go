package ai

import "github.com/duaraghav8/dockershrink/internal/models"

type OptimizeRequest struct {
	Dockerfile   string
	Dockerignore string
	PackageJSON  string

	DockerfileStageCount uint
	ProjectDirectory     *RestrictedFilesystem
}

type OptimizeResponse struct {
	Dockerfile      string
	Recommendations []*models.OptimizationAction
	ActionsTaken    []*models.OptimizationAction
}
