# GBR – GitLab Branch Ruler
A command-line tool to automatically protect default branches (`master`/`main`) across all projects in a specified GitLab group and its subgroups.


## Configuration
The tool reads settings from a JSON file (default `config.json`, or via `-config` flag).

Example `config.json`:
```json
{
  "gitlab_base_url": "https://gitlab.example.com",
  "gitlab_token": "YOUR_PERSONAL_ACCESS_TOKEN",
  "root_group_path": "myorg/backend",
  "push_access_level": 30,
  "merge_access_level": 30
}
```

- `gitlab_base_url`: URL of GitLab instance
- `gitlab_token`: your PAT with `api` scope
- `root_group_path`: full URL-encoded namespace path of the root group (e.g. `myorg/backend/subgroup`)
- `push_access_level`: access level for `push` (e.g. `30` for Developers)
- `merge_access_level`: access level for `merge` (e.g. `30` for Developers)


## Building and Running
The `./bin` directory contains application builds for a standard set of operating systems.

### Running a Built Application
The application can be run in two ways:
1) **With the flag**  
   `./app_name -config=./path/to/config.yaml` – allows specifying the configuration file location
2) **Without the flag**  
   `./app_name` – if the configuration file is located in the same directory as the executable

### Building from Source
To build and run from source, you need **Go** version `>=1.24.4` installed.

At the root of the repository, there is a bash script `build.sh` for building the application for the required OS and architecture.

#### Building for Standard OS Set
Run the script with arguments:
1) Build the application for the preset OSes and architectures — **windows amd64**; **linux amd64**; **macos amd64**; **arm64**
```shell
./build.sh all
```

2) Build the application for a specific OS. Available options — **windows**; **linux**; **macos**
```shell
./build.sh <OS>
```

#### Building for the Current OS
Run the script without arguments to build for the current OS and architecture:
```shell
./build.sh
```

#### Building for a Specific OS
If you need to build for an OS or architecture not included in the standard set, list available targets with:
```shell
go tool dist list
```
Then build with:

`Linux/macOS`
```shell
env GOOS=<OS> GOARCH=<architecture> go build -o <output_filename> main.go
```

`Windows/PowerShell`
```shell
$env:GOOS="<OS>"; $env:GOARCH="<architecture>"; go build -o <output_filename> main.go
```

`Windows/cmd`
```shell
set GOOS=<OS> && set GOARCH=<architecture> && go build -o <output_filename> main.go
```
