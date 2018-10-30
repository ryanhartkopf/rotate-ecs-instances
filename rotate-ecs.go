package main
import (
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ec2"
  "github.com/aws/aws-sdk-go/service/ecs"
  "flag"
  "fmt"
  "time"
)

func main() {
  // Retrieve and parse command line opts
  clusterPtr := flag.String("cluster", "", "ECS cluster name")
  regionPtr := flag.String("region", "us-west-2", "AWS region")
  flag.Parse()

  // Initialize the session
  sess, err := session.NewSession(&aws.Config{
    Region: aws.String(*regionPtr),
  })
  if err != nil {
    fmt.Println("Error creating session ", err)
    return
  }

  // Create the service clients
  ecsSvc := ecs.New(sess)
  ec2Svc := ec2.New(sess)

  // Run ECS describe-clusters
  clusters, err := ecsSvc.DescribeClusters(
    &ecs.DescribeClustersInput{
      Clusters: []*string{
        aws.String(*clusterPtr),
      },
    },
  )
  if err != nil {
    fmt.Println(err.Error())
    return
  } else if len(clusters.Clusters) > 0 {
    //fmt.Println("Cluster found:", *clusters.Clusters[0].ClusterName)
  } else if len(clusters.Failures) > 0 {
    fmt.Println("ECS describe-clusters failed with reason", *clusters.Failures[0].Reason)
    return
  } else {
    fmt.Println("ERROR: no clusters or failure reasons returned. Full response: ", clusters)
    return
  }

  // Run ECS list-container-instances
  containerInstances, err := ecsSvc.ListContainerInstances(
    &ecs.ListContainerInstancesInput {
      Cluster: aws.String(*clusterPtr),
    },
  )
  if err != nil {
    fmt.Println(err.Error())
    return
  } else if containerInstances != nil {
    //fmt.Println(containerInstances)
    //fmt.Println(len(containerInstances.ContainerInstanceArns))
  }

  // Loop over list of container instances
  for i := 0; i < len(containerInstances.ContainerInstanceArns); i++ {

    // Set variables
    count := 0
    sleep_timer := time.Duration(30)
    counter_limit := 60
    containerInstanceArn := *containerInstances.ContainerInstanceArns[i]

    // Get EC2 instance ID
    containerInstance, err := ecsSvc.DescribeContainerInstances(
      &ecs.DescribeContainerInstancesInput{
        Cluster: aws.String(*clusterPtr),
        ContainerInstances: []*string{
          aws.String(containerInstanceArn),
        },
      },
    )
    if err != nil {
      fmt.Println(err.Error())
      return
    }

    instanceId := containerInstance.ContainerInstances[0].Ec2InstanceId

    // Set ECS instance status to DRAINING
    _, err = ecsSvc.UpdateContainerInstancesState(
      &ecs.UpdateContainerInstancesStateInput{
        Cluster: aws.String(*clusterPtr),
        ContainerInstances: []*string{
          aws.String(containerInstanceArn),
        },
        Status: aws.String("DRAINING"),
      },
    )
    if err != nil {
      fmt.Println(err.Error())
      return
    } else {
      fmt.Println("Draining instance", *instanceId, "of running containers.")
    }

    // Check the running task count until it reaches 0 or the timer runs out
    for count < counter_limit {
      status, err := ecsSvc.DescribeContainerInstances(
        &ecs.DescribeContainerInstancesInput{
          Cluster: aws.String(*clusterPtr),
          ContainerInstances: []*string{
            aws.String(containerInstanceArn),
          },
        },
      )
      if err != nil {
        fmt.Println(err.Error())
        return
      } else if status.ContainerInstances[0].RunningTasksCount != nil {
        t := status.ContainerInstances[0].RunningTasksCount
        fmt.Println("Current task count is", *t)
        if *t == int64(0) { break }
        fmt.Println("Sleeping for", int(sleep_timer), "seconds...")
        time.Sleep(sleep_timer * time.Second)
        count++
      } else {
        fmt.Println("No error or running tasks count returned for ECS instance. Something is wrong.")
        return
      }
    }

    // Terminate ECS instance
    _, err = ec2Svc.TerminateInstances(
      &ec2.TerminateInstancesInput{
        InstanceIds: []*string{
          aws.String(*instanceId),
        },
      },
    )
    if err != nil {
      fmt.Println(err.Error())
    } else {
      fmt.Println("Terminated instance", *instanceId)
    }
  }
}
