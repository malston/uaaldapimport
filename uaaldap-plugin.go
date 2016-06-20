package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	uaa "github.com/cloudfoundry-incubator/uaa-token-fetcher"
	"github.com/cloudfoundry/cli/plugin"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
	"github.com/pivotalservices/uaaldapimport/config"
	"github.com/pivotalservices/uaaldapimport/token"
	"gopkg.in/yaml.v2"
)

func fatalIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stdout, "error:", err)
		os.Exit(1)
	}
}

func run(ctx *config.Context) {
	err := token.GetToken.MapUsers().AddUAAUsers().AddCCUsers().MapOrgs().MapSpaces(ctx)
	fatalIf(err)
}

func main() {
	plugin.Start(&UAALDAPPlugin{})
}

type UAALDAPPlugin struct{}

func (plugin UAALDAPPlugin) Run(cliConnection plugin.CliConnection, args []string) {
	// repo := NewRepository(cliConnection)
	users, env, err := ParseArgs(args)
	fatalIf(err)
	ctx, err := NewContext(users, env)
	fatalIf(err)
	run(ctx)
}

func (UAALDAPPlugin) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "uaaldapimport",
		Version: plugin.VersionType{
			Major: 0,
			Minor: 0,
			Build: 1,
		},
		Commands: []plugin.Command{
			{
				Name:     "import-ldap-users",
				HelpText: "Import users from LDAP into Cloud Foundry and assign them to Orgs and Spaces",
				UsageDetails: plugin.Usage{
					Usage: "$ cf import-ldap-users -u path/to/users.yml \\ \n \t-e path/to/environment.yml",
				},
			},
		},
	}
}

func ParseArgs(args []string) (string, string, error) {
	flags := flag.NewFlagSet("import-ldap-users", flag.ContinueOnError)
	users := flags.String("u", "config/fixtures/users.yml", "full path to users.yml")
	env := flags.String("e", "environment.yml", "full path to environment.yml")

	err := flags.Parse(args[1:])
	if err != nil {
		return "", "", err
	}

	return *users, *env, nil
}

func NewContext(users, env string) (*config.Context, error) {
	logger := lager.NewLogger("uaaldapimport")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	ctx := &config.Context{}
	ctx.Logger = logger

	usersFile, err := os.Open(users)
	if err != nil {
		err = fmt.Errorf("Cannot open %s : %s", users, err.Error())
		return nil, err
	}

	cfg, err := config.Parse(usersFile)
	fatalIf(err)

	envFile, err := os.Open(env)
	if err != nil {
		err = fmt.Errorf("Cannot open %s : %s", env, err.Error())
		return nil, err
	}

	data, err := ioutil.ReadAll(envFile)
	fatalIf(err)

	err = yaml.Unmarshal(data, ctx)
	fatalIf(err)

	ctx.RequestFn = token.RequestWithToken
	ctx.Users = cfg.Users

	tokenFetcherConfig := uaa.TokenFetcherConfig{
		MaxNumberOfRetries:                3,
		RetryInterval:                     15 * time.Second,
		ExpirationBufferTime:              30,
		DisableTLSCertificateVerification: true,
	}
	oauth := &uaa.OAuthConfig{
		TokenEndpoint: ctx.UAAURL,
		ClientName:    ctx.Clientid,
		ClientSecret:  ctx.Secret,
		Port:          443,
	}
	clk := clock.NewClock()
	fetcher, err := uaa.NewTokenFetcher(ctx.Logger, oauth, tokenFetcherConfig, clk)
	fatalIf(err)
	ctx.TokenFetcher = fetcher
	return ctx, nil
}

type Repository struct {
	conn plugin.CliConnection
}

func NewRepository(conn plugin.CliConnection) *Repository {
	return &Repository{
		conn: conn,
	}
}
