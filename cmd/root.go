/*
Copyright © 2022 Aatman <aatman@auroville.org.in>

*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Hunter-Thompson/viper"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v35/github"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

type pullRequestOptions struct {
	commitMessage   string
	branchName      string
	pullRequestBody string
	repositoryName  string
	autoMergeLabel  string
	githubHubOrg    string
	headBranchName  string
}

type cloneOptions struct {
	repoURI        string
	branchName     string
	refName        string
	githubUsername string
}

var (
	imageID             string
	repoName            string
	repoToClone         string
	custom              bool
	imagePrefix         string
	configFolder        string
	configName          string
	viperSearch         string
	configType          string
	appendCommitMessage string
	updateImage         bool
	updateVersion       bool
	githubOrg           string
	githubUsername      string
	githubEmail         string
	autoMergeLabel      string
	headBranchName      string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "go-image-update",
	Short: "Updates a JSON/YAML config with a new version and creates a pull request",
	Run: func(cmd *cobra.Command, args []string) {
		vars := map[string]string{
			"imageid":        imageID,
			"repobame":       repoName,
			"repoclone":      repoToClone,
			"imageprefix":    imagePrefix,
			"configfolder":   configFolder,
			"vipersearch":    viperSearch,
			"configtype":     configType,
			"configname":     configName,
			"commitmessage":  appendCommitMessage,
			"githuborg":      githubOrg,
			"githubemail":    githubEmail,
			"githubusername": githubUsername,
			"headbranchname": headBranchName,
		}

		for k, v := range vars {
			if v == "" {
				log.Fatal().Msgf("%s not set", k)
			}
		}

		if configType != "yaml" && configType != "json" {
			log.Fatal().Msg("configtype must be yaml or json")
		}

		log.Info().Msgf("updating image %s for %s by cloning %s and searching for %s", imageID, repoName, repoToClone, viperSearch)
		updateConfig()
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVar(&imageID, "imageid", "", "Image ID to use")
	rootCmd.Flags().StringVar(&repoName, "reponame", "", "Repo of image")
	rootCmd.Flags().StringVar(&repoToClone, "repoclone", "", "Repo to clone")
	rootCmd.Flags().StringVar(&imagePrefix, "imageprefix", "", "Image prefix")
	rootCmd.Flags().StringVar(&configFolder, "configfolder", "", "Config folder")
	rootCmd.Flags().StringVar(&viperSearch, "vipersearch", "", "Viper search keyword")
	rootCmd.Flags().StringVar(&configType, "configtype", "", "Config type - json/yaml")
	rootCmd.Flags().StringVar(&configName, "configname", "", "Config name")
	rootCmd.Flags().StringVar(&appendCommitMessage, "commitmessage", "", "Commit message to append to feat(repo_name): ")

	rootCmd.Flags().BoolVar(&updateImage, "updateimage", true, "Update image with image prefix and version")
	rootCmd.Flags().BoolVar(&updateVersion, "updateversion", false, "Update version only")
	rootCmd.Flags().BoolVar(&custom, "custom", false, "Custom image")

	rootCmd.Flags().StringVar(&githubOrg, "githuborg", "", "github organization")
	rootCmd.Flags().StringVar(&githubUsername, "githubusername", "", "github username")
	rootCmd.Flags().StringVar(&githubEmail, "githubemail", "", "github email")
	rootCmd.Flags().StringVar(&autoMergeLabel, "automergelabel", "", "label to add to the pr")
	rootCmd.Flags().StringVar(&headBranchName, "headbranchname", "", "Branch to create a PR on")

	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func updateConfig() {
	branch := fmt.Sprintf("refs/heads/%s-%s", repoName, imageID)
	repoURI := fmt.Sprintf("https://github.com/%s/%s.git", githubOrg, repoToClone)

	tempFolder, workTree, clone, auth, err := cloneAndSet(cloneOptions{
		repoURI:        repoURI,
		branchName:     branch,
		refName:        headBranchName,
		githubUsername: githubUsername,
	})
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	defer os.RemoveAll(tempFolder)

	folder := fmt.Sprintf("%s%s", tempFolder, configFolder)
	image := fmt.Sprintf("%s/%s:%s", imagePrefix, repoName, imageID)

	if custom {
		viper.Reset()
		viper.SetKeysCaseSensitive(true)
		viper.AddConfigPath(folder)
		viper.SetConfigName(configName)
		viper.SetConfigType(configType)

		if err := viper.ReadInConfig(); err != nil {
			log.Fatal().Err(err).Send()
		}

		i := viper.Get(viperSearch)

		if updateImage {
			if i == image {
				log.Info().Msgf("image %s already set", image)
				os.Exit(0)
			}

			viper.Set(viperSearch, image)
		}

		if updateVersion {
			if i == imageID {
				log.Info().Msgf("version %s already set", imageID)
				os.Exit(0)
			}

			viper.Set(viperSearch, imageID)
		}

		err := viper.WriteConfig()
		if err != nil {
			log.Fatal().Err(err).Send()
		}
	}

	_, err = workTree.Add(".")
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	cm := fmt.Sprintf("feat(%s): %s", repoName, appendCommitMessage)

	_, err = workTree.Commit(cm, &git.CommitOptions{
		Author: &object.Signature{
			Name:  githubUsername,
			Email: githubEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	err = clone.Push(&git.PushOptions{
		Auth:       auth,
		RemoteName: "origin",
	})
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	body := fmt.Sprintf(`Link to changes if tag:  https://github.com/%s/%s/releases/tag/%s
Link to changes if commit: https://github.com/%s/%s/commit/%s`, githubOrg, repoName, imageID, githubOrg, repoName, imageID)

	pr, err := createPR(pullRequestOptions{
		repositoryName:  repoToClone,
		commitMessage:   cm,
		branchName:      branch,
		pullRequestBody: body,
		githubHubOrg:    githubOrg,
		autoMergeLabel:  autoMergeLabel,
		headBranchName:  headBranchName,
	})
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	log.Info().Msgf("PR created: %s", pr)
}

func cloneAndSet(r cloneOptions) (string, *git.Worktree, *git.Repository, *http.BasicAuth, error) {
	tempFolder := "/tmp/" + r.branchName + "/"

	auth := &http.BasicAuth{
		Username: r.githubUsername,
		Password: os.Getenv("GIT_TOKEN"),
	}

	rn := fmt.Sprintf("refs/heads/%s", r.refName)

	clone, err := git.PlainClone(tempFolder, false, &git.CloneOptions{
		URL:           r.repoURI,
		Progress:      os.Stdout,
		Auth:          auth,
		ReferenceName: plumbing.ReferenceName(rn),
	})
	if err != nil {
		return tempFolder, nil, nil, nil, err
	}

	workTree, err := clone.Worktree()
	if err != nil {
		return tempFolder, nil, nil, nil, err
	}

	headRef, err := clone.Head()
	if err != nil {
		return tempFolder, nil, nil, nil, err
	}

	x := plumbing.ReferenceName(r.branchName)

	ref := plumbing.NewHashReference(x, headRef.Hash())

	err = clone.Storer.SetReference(ref)

	if err != nil {
		return tempFolder, nil, nil, nil, err
	}

	err = workTree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(r.branchName),
	})

	if err != nil {
		return tempFolder, nil, nil, nil, err
	}

	return tempFolder, workTree, clone, auth, nil
}

func createPR(pr pullRequestOptions) (string, error) {
	ctx := context.Background()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GIT_TOKEN")},
	)

	tc := oauth2.NewClient(ctx, ts)

	githubClient := github.NewClient(tc)

	newPR := &github.NewPullRequest{
		Title: &pr.commitMessage,
		Head:  &pr.branchName,
		Base:  &pr.headBranchName,
		Body:  &pr.pullRequestBody,
	}

	pullRequest, _, err := githubClient.PullRequests.Create(ctx, pr.githubHubOrg, pr.repositoryName, newPR)
	if err != nil {
		return "", fmt.Errorf("pull request creation failed: %w", err)
	}

	if pr.autoMergeLabel != "" {
		label := []string{
			pr.autoMergeLabel,
		}

		_, _, err = githubClient.Issues.ReplaceLabelsForIssue(ctx, pr.githubHubOrg, pr.repositoryName, pullRequest.GetNumber(), label)
		if err != nil {
			return "", err
		}
	}

	return pullRequest.GetHTMLURL(), nil
}
