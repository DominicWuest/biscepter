/*
Package biscepter provides a Go interface for creating and running bisection jobs.

Jobs can most easily be created by passing in a job config to [GetJobFromConfig], but can also be created manually by populating a [Job] struct.
For a manually created job to work, at least the following fields have to be populated:
  - ReplicaCount
  - GoodCommit & BadCommit
  - Dockerfile or DockerfilePath
  - Repository

Every replica represents one issue to be bisected, meaning that the ReplicaCount parameter symbolizes how many issues should be bisected concurrently.

After a job struct was acquired, the job can be started using [Job.Run].

The [Job.Run] function returns two channels.
The first of of these channels contains [RunningSystem]-s, which are to be used to determine whether a certain commit is good or bad using the [RunningSystem.IsGood] and [RunningSystem.IsBad] methods.
The latter channel contains [OffendingCommit]-s, which represent a completed bisection and contain information about the offending commit of the bisected issue.

When all issues have been diagnosed and an [OffendingCommit] was received for each one of them, the job can be stopped using [Job.Stop], which will shutdown all running docker containers.
*/
package biscepter
