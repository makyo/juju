// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package cloud

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/juju/cmd"
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"launchpad.net/gnuflag"

	jujucloud "github.com/juju/juju/cloud"
	"github.com/juju/juju/cmd/juju/common"
)

var logger = loggo.GetLogger("juju.cmd.juju.cloud")

type listCloudsCommand struct {
	cmd.CommandBase
	out cmd.Output
}

// listCloudsDoc is multi-line since we need to use ` to denote
// commands for ease in markdown.
var listCloudsDoc = "" +
	"Provided information includes 'cloud' (as understood by Juju), cloud\n" +
	"'type', and cloud 'regions'.\n" +
	"The listing will consist of public clouds and any custom clouds made\n" +
	"available through the `juju add-cloud` command. The former can be updated\n" +
	"via the `juju update-cloud` command.\n" +
	"By default, the tabular format is used.\n" + listCloudsDocExamples

var listCloudsDocExamples = `
Examples:

    juju list-clouds

See also: show-cloud
          update-clouds
          add-cloud
`

// NewListCloudsCommand returns a command to list cloud information.
func NewListCloudsCommand() cmd.Command {
	return &listCloudsCommand{}
}

func (c *listCloudsCommand) Info() *cmd.Info {
	return &cmd.Info{
		Name:    "list-clouds",
		Purpose: "Lists all clouds available to Juju.",
		Doc:     listCloudsDoc,
	}
}

func (c *listCloudsCommand) SetFlags(f *gnuflag.FlagSet) {
	c.out.AddFlags(f, "tabular", map[string]cmd.Formatter{
		"yaml":    cmd.FormatYaml,
		"json":    cmd.FormatJson,
		"tabular": formatCloudsTabular,
	})
}

const localPrefix = "local:"

func (c *listCloudsCommand) Run(ctxt *cmd.Context) error {
	details, err := getCloudDetails()
	if err != nil {
		return err
	}
	return c.out.Write(ctxt, details)
}

func getCloudDetails() (map[string]*cloudDetails, error) {
	clouds, _, err := jujucloud.PublicCloudMetadata(jujucloud.JujuPublicCloudsPath())
	if err != nil {
		return nil, err
	}
	details := make(map[string]*cloudDetails)
	for name, cloud := range clouds {
		cloudDetails := makeCloudDetails(cloud)
		details[name] = cloudDetails
	}

	// Add in built in providers like "lxd" and "manual".
	for name, cloud := range common.BuiltInProviders() {
		cloudDetails := makeCloudDetails(cloud)
		cloudDetails.Source = "built-in"
		details[name] = cloudDetails
	}

	personalClouds, err := jujucloud.PersonalCloudMetadata()
	if err != nil {
		return nil, err
	}
	for name, cloud := range personalClouds {
		// Add to result with "local:" prefix.
		cloudDetails := makeCloudDetails(cloud)
		cloudDetails.Source = "local"
		details[localPrefix+name] = cloudDetails
	}
	return details, nil
}

// Public clouds sorted first, then personal ie has a prefix of "local:".
type cloudSourceOrder []string

func (a cloudSourceOrder) Len() int      { return len(a) }
func (a cloudSourceOrder) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a cloudSourceOrder) Less(i, j int) bool {
	isLeftLocal := strings.HasPrefix(a[i], localPrefix)
	isRightLocal := strings.HasPrefix(a[j], localPrefix)
	if isLeftLocal == isRightLocal {
		return a[i] < a[j]
	}
	return isRightLocal
}

// formatCloudsTabular returns a tabular summary of cloud information.
func formatCloudsTabular(value interface{}) ([]byte, error) {
	clouds, ok := value.(map[string]*cloudDetails)
	if !ok {
		return nil, errors.Errorf("expected value of type %T, got %T", clouds, value)
	}

	// For tabular we'll sort alphabetically, user clouds last.
	var cloudNames []string
	for name, _ := range clouds {
		cloudNames = append(cloudNames, name)
	}
	sort.Sort(cloudSourceOrder(cloudNames))

	var out bytes.Buffer
	const (
		// To format things into columns.
		minwidth = 0
		tabwidth = 1
		padding  = 2
		padchar  = ' '
		flags    = 0
	)
	tw := tabwriter.NewWriter(&out, minwidth, tabwidth, padding, padchar, flags)
	p := func(values ...string) {
		text := strings.Join(values, "\t")
		fmt.Fprintln(tw, text)
	}
	p("CLOUD\tTYPE\tREGIONS")
	for _, name := range cloudNames {
		info := clouds[name]
		var regions []string
		for _, region := range info.Regions {
			regions = append(regions, fmt.Sprint(region.Key))
		}
		// TODO(wallyworld) - we should be smarter about handling
		// long region text, for now we'll display the first 7 as
		// that covers all clouds except AWS and Azure and will
		// prevent wrapping on a reasonable terminal width.
		regionCount := len(regions)
		if regionCount > 7 {
			regionCount = 7
		}
		regionText := strings.Join(regions[:regionCount], ", ")
		if len(regions) > 7 {
			regionText = regionText + " ..."
		}
		p(name, info.CloudType, regionText)
	}
	tw.Flush()

	return out.Bytes(), nil
}
