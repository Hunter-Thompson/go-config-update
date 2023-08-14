/*
Copyright © 2022 Aatman <aatman@auroville.org.in>

*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
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

type commentOptions struct {
	gitRef    string
	gitHubOrg string
	appName   string
	comment   string
}

var (
	imageID             string
	repoName            string
	repoToClone         string
	custom              bool
	imagePrefix         string
	configFolder        string
	configNames         []string
	viperSearch         []string
	gitRef              string
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
			"reponame":       repoName,
			"repoclone":      repoToClone,
			"imageprefix":    imagePrefix,
			"configfolder":   configFolder,
			"configtype":     configType,
			"commitmessage":  appendCommitMessage,
			"githuborg":      githubOrg,
			"githubemail":    githubEmail,
			"githubusername": githubUsername,
			"headbranchname": headBranchName,
			"gitRef":         gitRef,
		}

		for k, v := range vars {
			if v == "" {
				log.Fatal().Msgf("%s not set", k)
			}
		}

		mustHaveValues(viperSearch, "vipersearch")
		mustHaveValues(configNames, "confignames")

		if len(configNames) != len(viperSearch) {
			log.Fatal().Msgf("confignames, and vipersearch must have the same number of values")
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
	rootCmd.Flags().StringSliceVar(&viperSearch, "vipersearch", []string{}, "Viper search keywords")
	rootCmd.Flags().StringVar(&configType, "configtype", "", "Config type - json/yaml")
	rootCmd.Flags().StringSliceVar(&configNames, "confignames", []string{}, "Config names")
	rootCmd.Flags().StringVar(&appendCommitMessage, "commitmessage", "", "Commit message to append to feat(repo_name): ")

	rootCmd.Flags().BoolVar(&updateImage, "updateimage", true, "Update image with image prefix and version")
	rootCmd.Flags().BoolVar(&updateVersion, "updateversion", false, "Update version only")
	rootCmd.Flags().BoolVar(&custom, "custom", false, "Custom image")

	rootCmd.Flags().StringVar(&githubOrg, "githuborg", "", "github organization")
	rootCmd.Flags().StringVar(&githubUsername, "githubusername", "", "github username")
	rootCmd.Flags().StringVar(&githubEmail, "githubemail", "", "github email")
	rootCmd.Flags().StringVar(&autoMergeLabel, "automergelabel", "", "label to add to the pr")
	rootCmd.Flags().StringVar(&headBranchName, "headbranchname", "", "Branch to create a PR on")
	rootCmd.Flags().StringVar(&gitRef, "gitRef", "", "git ref of image")

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
		viper.SetConfigType(configType)

		for n, configName := range configNames {
			viper.SetConfigName(configName)
			if err := viper.ReadInConfig(); err != nil {
				log.Fatal().Err(err).Send()
			}

			i := viper.Get(viperSearch[n])

			if updateImage {
				if i == image {
					log.Info().Msgf("image %s already set", image)
					os.Exit(0)
				}

				viper.Set(viperSearch[n], image)
			}

			if updateVersion {
				if i == imageID {
					log.Info().Msgf("version %s already set", imageID)
					os.Exit(0)
				}

				viper.Set(viperSearch[n], imageID)
			}

			err := viper.WriteConfig()
			if err != nil {
				log.Fatal().Err(err).Send()
			}
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

	log.Info().Msgf("creating comment on commit")

	err = createComment(commentOptions{
		appName:   repoName,
		gitHubOrg: githubOrg,
		gitRef:    gitRef,
		comment:   pr,
	})

	if err != nil {
		log.Fatal().Err(err).Send()
	}
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

func createComment(co commentOptions) error {
	ctx := context.Background()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GIT_TOKEN")},
	)

	tc := oauth2.NewClient(ctx, ts)

	comment := &github.RepositoryComment{
		Body: github.String(co.comment),
	}

	githubClient := github.NewClient(tc)

	r, _ := regexp.Compile(`\b[0-9a-f]{5,40}\b`)
	commit := ""

	if r.MatchString(co.gitRef) {
		log.Info().Msgf("matched commit regex, commit: %s", co.gitRef)
		commit = co.gitRef
	} else {
		log.Info().Msgf("didnt matched commit regex, commit: %s", co.gitRef)
		opt := &github.ListOptions{
			PerPage: 10,
		}

		var allTags []*github.RepositoryTag
		for {
			tags, resp, err := githubClient.Repositories.ListTags(ctx, co.gitHubOrg, co.appName, opt)
			if err != nil {
				return err
			}
			allTags = append(allTags, tags...)
			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}

		for _, tag := range allTags {
			if tag.GetName() == co.gitRef {
				commit = tag.GetCommit().GetSHA()
			}
		}
	}

	_, _, err := githubClient.Repositories.CreateComment(ctx, co.gitHubOrg, co.appName, commit, comment)
	if err != nil {
		return err
	}

	return nil
}

func mustHaveValues(s []string, name string) {
	if len(s) == 0 {
		log.Fatal().Msgf("%s has no values", name)
	}
}
