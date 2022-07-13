# tekton-create-namespace (tcn)

## description
tekton-create-namespace is intended to be used as a Tekton task to allow a Tekton pipeline user to provide a full-blown
dynamic environment. However, it could also be used as a standalone binary/docker image and integrated somewhere else.
To facilitate management of the environments, provisioning and cleanup is provided at the namespace level,
ensuring that no hanging resources interfere with the pipeline process.

**IMPORTANT NOTES**:
- As this also deletes namespaces, this is only recommended for dynamically spawning dev environments!
- As this also deletes namespaces, make sure that [<prefix>-]<namespace> only matches namespaces that are safe to 
  delete!

`tcn` takes care of
* lifecycle of namespaces (create/delete)
  * create: deletes old namespaces, if available and then creates a new one
  * delete: only delete old namespaces
  * note that both create and delete operations are controlled via separate invocations of the `tcn` app, see
    the mode parameter in [usage](#usage)!
* user authorization to the namespace

## state of project


## Usage
```text
Usage of ./tcn:
  -level string
        Log level: panic|fatal|error|warn|info|debug|trace (default "info")
  -mode string
        Mandatory: create|delete. Note, that create will first delete the previous namespace as well!
        delete: deletes namspaces matching '[<prefix>-]<namespace>', but not if the namespace already exists with
                the same suffix.
        create: same as delete + creates a new one afterwards (default "create")
  -namespace string
        Mandatory: input parameter for the namespace name. Notice that the full pattern of the (output) namespace is: 
        [<prefix>-]<namespace>[-<suffix>]
  -outFilePath string
        If specified, will write the full output namespace to this file path
  -prefix string
        Optional: Prefix of namespace (default "tcn")
  -suffix string
        Optional: Suffix of namespace. If empty, a random string of  characters will be appended
  -user string
        Optional: user that gets authorized in the created namespace
```

## Build

```bash
go build -o tcn .
```

## Local container build

```bash
docker build . -t tcn
nerdctl build . -t tcn
```
