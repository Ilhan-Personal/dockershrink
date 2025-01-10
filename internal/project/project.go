package project

import (
	"fmt"
	"strings"

	"github.com/duaraghav8/dockershrink/internal/ai"
	"github.com/duaraghav8/dockershrink/internal/dockerfile"
	"github.com/duaraghav8/dockershrink/internal/dockerignore"
	"github.com/duaraghav8/dockershrink/internal/models"
	"github.com/duaraghav8/dockershrink/internal/packagejson"
	"github.com/duaraghav8/dockershrink/internal/restrictedfilesystem"
)

type Project struct {
	dockerfile   *dockerfile.Dockerfile
	dockerignore *dockerignore.Dockerignore
	packageJSON  *packagejson.PackageJSON

	recommendations []*models.OptimizationAction
	actionsTaken    []*models.OptimizationAction

	directory *restrictedfilesystem.RestrictedFilesystem
}

func NewProject(dockerfile *dockerfile.Dockerfile, dockerignore *dockerignore.Dockerignore, packageJson *packagejson.PackageJSON, directory *restrictedfilesystem.RestrictedFilesystem) *Project {
	return &Project{
		dockerfile:      dockerfile,
		dockerignore:    dockerignore,
		packageJSON:     packageJson,
		directory:       directory,
		recommendations: []*models.OptimizationAction{},
		actionsTaken:    []*models.OptimizationAction{},
	}
}

func (p *Project) OptimizeDockerImage(aiService *ai.AIService) (*OptimizationResponse, error) {
	// Ensure that .dockerignore exists and contains the recommended entries
	if !p.dockerignore.Exists() {
		p.dockerignore.Create()
		action := &models.OptimizationAction{
			Rule:        "create-dockerignore",
			Filename:    ".dockerignore",
			Title:       "Created .dockerignore file",
			Description: "Created a new .dockerignore file to exclude unnecessary files & folders from the Docker build context.",
		}
		p.addActionTaken(action)
	}

	entries := []string{"node_modules", "npm_debug.log", ".git"}
	added := p.dockerignore.AddIfNotPresent(entries)
	if len(added) > 0 {
		action := &models.OptimizationAction{
			Rule:        "update-dockerignore",
			Filename:    ".dockerignore",
			Title:       "Updated .dockerignore file",
			Description: fmt.Sprintf("Added the following entries to .dockerignore to exclude them from the Docker build context:\n%s", strings.Join(added, "\n")),
		}
		p.addActionTaken(action)
	}

	originalDockerfile := p.dockerfile

	if aiService != nil {
		req := &ai.OptimizeRequest{
			Dockerfile:           p.dockerfile.Raw(),
			Dockerignore:         p.dockerignore.Raw(),
			PackageJSON:          p.packageJSON.String(),
			ProjectDirectory:     p.directory,
			DockerfileStageCount: p.dockerfile.GetStageCount(),
		}
		resp, err := aiService.OptimizeDockerfile(req)
		if err != nil {
			return nil, fmt.Errorf("AI service failed to optimize Dockerfile: %w", err)
		}

		p.dockerfile, err = dockerfile.NewDockerfile(strings.NewReader(resp.Dockerfile))
		if err != nil {
			return nil, fmt.Errorf("Failed to parse Dockerfile returned by AI service: %w", err)
		}

		for _, r := range resp.Recommendations {
			p.addRecommendation(r)
		}
		for _, a := range resp.ActionsTaken {
			p.addActionTaken(a)
		}
	}

	// Only check for the final stage's base image if it was not changed by AI

	origStageCount := originalDockerfile.GetStageCount()
	newStageCount := p.dockerfile.GetStageCount()
	origFinalStageBaseImage := originalDockerfile.GetFinalStage().BaseImage()
	newFinalStageBaseImage := p.dockerfile.GetFinalStage().BaseImage()

	if origStageCount == newStageCount && origFinalStageBaseImage.Equals(newFinalStageBaseImage) {
		p.finalStageLightBaseImage()
	}

	return &OptimizationResponse{
		Dockerfile:      p.dockerfile.Raw(),
		Dockerignore:    p.dockerignore.Raw(),
		ActionsTaken:    p.actionsTaken,
		Recommendations: p.recommendations,
	}, nil
}

func (p *Project) addRecommendation(r *models.OptimizationAction) {
	p.recommendations = append(p.recommendations, r)
}

func (p *Project) addActionTaken(a *models.OptimizationAction) {
	p.actionsTaken = append(p.actionsTaken, a)
}
