# Community Questions

Questions raised by viewers on the [Lecture 1 post](https://x.com/SinghDevHub/status/2109654069325472138). Each one will be addressed in an upcoming lecture.

## Agent loop: failure handling

### 1. A tool call dies halfway, but the loop carries on as if it worked
> "Most agent bugs I hit were a tool call dying halfway while the loop carried on as if it worked. Does the course cover how the loop handles a failed call, or only the happy path?"
>
> — [@dipenvaja](https://x.com/dipenvaja/status/2109663818012110904)

**Status:** to be addressed

### 2. An empty tool result is read as success
> "A tool returning an empty result that the loop read as success. Silent failures hurt me more than loud ones, so it would help to show how the loop tells the two apart."
>
> — [@dipenvaja](https://x.com/dipenvaja/status/2109682671559569867)

**Status:** to be addressed

### 3. The model keeps retrying the same failing tool call
> "If shell or file_read returns a raw error string, the model often retries the same call. A structured result ({ok:false, retryable:false, reason}) plus a per-tool call cap ends those loops early."
>
> — [@akhilnalliboina](https://x.com/akhilnalliboina/status/2109883840235708784)

**Status:** to be addressed

### 4. A relative path resolves from the wrong working directory
> "A relative path resolved from the wrong cwd, so file_read 'succeeds' on the wrong file. Fix: return the resolved path."
>
> — [@akhilnalliboina](https://x.com/akhilnalliboina/status/2110730953727344801)

**Status:** to be addressed

### 5. Shell commands that wait for input hang until timeout
> "Shell commands that wait for input (a y/n prompt) hang until timeout. Fix: run shell with stdin closed."
>
> — [@akhilnalliboina](https://x.com/akhilnalliboina/status/2110730953727344801)

**Status:** to be addressed