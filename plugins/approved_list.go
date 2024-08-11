package plugins

type ApprovedRepos struct {
	RepositoryOwner string
	RepositoryName  string
	BranchName      string
}

var ThirdPartyApprovedPlugins = []*ApprovedRepos{
	{
		RepositoryOwner: "apito-cms",
		RepositoryName:  "plugin-email-auth",
		BranchName:      "main",
	},
	{
		RepositoryOwner: "apito-cms",
		RepositoryName:  "plugin-phone-auth",
		BranchName:      "main",
	},
	{
		RepositoryOwner: "apito-cms",
		RepositoryName:  "plugin-s3-file-upload",
		BranchName:      "main",
	},
}
