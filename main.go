package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/eparis/bugtool/pkg/api"
	"github.com/eparis/bugzilla"
	"github.com/ghodss/yaml"
	//"github.com/kr/pretty"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func getBugActions() ([]api.BugAction, error) {
	var pathNames []string

	root := "operations"
	err := filepath.Walk(root, func(pathName string, info os.FileInfo, err error) error {
		if path.Ext(pathName) != ".yaml" {
			return nil
		}
		pathNames = append(pathNames, pathName)
		return nil
	})
	if err != nil {
		return nil, err
	}
	actions := []api.BugAction{}
	for _, path := range pathNames {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}

		action := api.BugAction{}
		err = yaml.Unmarshal(data, &action)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, nil
}

// bugzillaClient returns a Bugzilla client.
func bugzillaClient() (bugzilla.Client, error) {
	endpoint := "https://bugzilla.redhat.com"

	//FIXME fetch the key via an arg
	dat, err := ioutil.ReadFile("apikey")
	if err != nil {
		return nil, err
	}
	apikey := strings.TrimRight(string(dat), "\r\n")

	var generator *func() []byte
	generatorFunc := func() []byte {
		return []byte(apikey)
	}
	generator = &generatorFunc

	return bugzilla.NewClient(*generator, endpoint), nil
}

type actionNames []string

func (an actionNames) Has(name string) bool {
	for _, actionName := range an {
		if name == actionName {
			return true
		}
	}
	return false
}

func doBug(cmd *cobra.Command) error {
	client, err := bugzillaClient()
	if err != nil {
		return err
	}

	actionSlice, err := cmd.Flags().GetStringSlice("actions")
	if err != nil {
		return err
	}
	selectedActions := actionNames(actionSlice)
	logrus.Infof("Running: %v", selectedActions)

	potentialActions, err := getBugActions()
	if err != nil {
		return err
	}

	actions := []api.BugAction{}
	for i := range potentialActions {
		potentialAction := potentialActions[i]
		if len(selectedActions) == 0 && potentialAction.Default {
			actions = append(actions, potentialAction)
			continue
		}
		if selectedActions.Has(potentialAction.Name) {
			actions = append(actions, potentialAction)
			continue
		}
	}
	for _, action := range actions {
		query := action.Query
		bugs, err := client.Search(query)
		if err != nil {
			return err
		}
		logrus.Infof("%q will update %d bugs", action.Description, len(bugs))

		update := action.Update
		for _, bug := range bugs {
			logrus.Infof("Updating %d", bug.ID)
			err := client.UpdateBug(bug.ID, update)
			if err != nil {
				logrus.Infof("Unable to update %d: %v", bug.ID, err)
			}
		}
	}
	return nil
}

func main() {
	cmd := &cobra.Command{
		Use: filepath.Base(os.Args[0]),
		RunE: func(cmd *cobra.Command, _ []string) error {
			err := doBug(cmd)
			if err != nil {
				return err
			}
			return nil
		},
	}
	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	cmd.Flags().StringSlice("actions", []string{}, "Actions to run, unset runs all actions with default=true")
	cmd.Execute()
}
