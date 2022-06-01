# tekton-create-namespace (tcn)

tcn stands for Tekton Create Namespace and is a lightweight Go application designed to facilitate the creation of namespace as a Tekton step in a Tekton pipeline.

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
