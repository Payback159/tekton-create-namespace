# tekton-create-namespace (tcn)

tekton-create-namespace is intended to be used as a Tekton task to allow a Tekton pipeline user to provide a full blown environment. To facilitate management of the environments, provisioning and cleanup is provided at the namespace level, ensuring that no hanging resources interfere with the pipeline process.

tcn takes care of

* the creation and lifecycle of namespaces.
* unique mapping of namespaces to branch name and build hash/number.
* Optionally, the developer who triggered the Tekton pipeline can be automatically authorized to the namespace to be able to analyze in case of failure.

## Build

```bash
go build -o tcn .
```

## Local container build

```bash
docker build . -t tcn
nerdctl build . -t tcn
```
