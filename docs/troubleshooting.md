# Troubleshooting

## Mac M1 Docker Container Execution Failure

If you are running on a Mac M1, and are getting an error similar to:

```
ERR execution failure error="input:1: container.from.withEnvVariable.withExec.stdout process \"echo sample output from debug container\" did not complete successfully: exit code: 1\n\nStdout:\n\nStderr:\n"
```

You may need to install [colima](https://github.com/abiosoft/colima).

To install colima on a Mac using Homebrew:

```
brew install colima
```

Start colima:

```
colima start --arch x86_64
```
Then go ahead and run the portage.
