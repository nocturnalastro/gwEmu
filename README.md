POC for injecting stressors into a footprint to generate a workload. 

#### Usage
```bash
$ go build
```

```bash 
$ ./gwEmu -f deployment.yaml
```
if you wish to select one of the sub versions 

```bash
$ ./gwEmu -f deployment.yaml -p N
```

where N is the number in the prefix for instance `gwEmu-1` would be `-p 1`
