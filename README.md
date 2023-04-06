# IterationCapacity

## Overview

IterationCapacity is a Go program that retrieves data from Azure DevOps to provide insights into a project team's velocity over time. It creates a 'data.sqlite' file (database) to store all the data it retrieves from Azure DevOps regarding a project's team capacity and attempts to create an insight into a team's velocity over time.

## Dependencies

To run IterationCapacity, you will need:

- Go (version 1.16 or later)
- The following Go packages:
  - "github.com/microsoft/azure-devops-go-api/azuredevops"
  - "github.com/microsoft/azure-devops-go-api/azuredevops/work"
  - "github.com/mattn/go-sqlite3"

## Installation

To install IterationCapacity, follow these steps:

- Clone this repository to your local machine.
- Install the required dependencies by running the following command:

```golang
go get -u github.com/microsoft/azure-devops-go-api/azuredevops github.com/microsoft/azure-devops-go-api/azuredevops/work github.com/mattn/go-sqlite3
```

- In the root directory of the repository, run the following command to build the program:

```golang
go build
```

## Usage

Before running IterationCapacity, you will need to create an **`arguments.json`** file in the root directory of the repository. This file should contain the following information:

```json
{
	"orgURL": "https://dev.azure.com/<YourOrg>",
	"token": "<PersonalAccessToken>",
	"project": "<YourProject>",
	"team": "<YourTeam>",
	"sprintStart": 67
}
```

Replace `<PersonalAccessToken>`, `<YourOrg>`, `<YourProject>`, `<YourTeam>`, and `67` with the relevant information for your project.

Once you have created the **`arguments.json`** file, you can run IterationCapacity by executing the following command in the root directory of the repository:

```cmdshell
./IterationCapacity
```

This will create a 'data.sqlite' file (database) in the root directory of the repository that contains all the data retrieved from Azure DevOps.

## Troubleshooting

If you encounter any issues when running IterationCapacity, please check the following:

- Ensure that you have installed all the necessary dependencies.
- Check that the **`arguments.json`** file contains the correct information for your project.
- If you are still experiencing issues, please consult the Go documentation or seek help from the Go community.

## Limitations

- This program only works with Azure DevOps as a data source.
- The program is currently hardcoded to use 'data.sqlite' as the database filename.
