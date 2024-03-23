#!/bin/bash

# Variables
CLUSTER_NAME="CLUSTER_NAME" # Change this to your EKS cluster name
NODE_GROUP_NAME="k8s-autoscaler-benchmarker-ng"
INSTANCE_TYPE="t3.medium"
ROLE_ARN="arn:aws:iam::ACCOUNT_ID:role/ROLE_NAME" # Update ACCOUNT_ID and ROLE_NAME for Node Group
AMI_TYPE="BOTTLEROCKET_x86_64"
CAPACITY_TYPE="SPOT"
DESIRED_SIZE=0
MIN_SIZE=0
MAX_SIZE=10
LABEL_KEY="eks.autify.com/k8s-autoscaler-benchmarker"
LABEL_VALUE="true"
TAINT_KEY="eks.autify.com/k8s-autoscaler-benchmarker"
TAINT_VALUE=""
TAINT_EFFECT="NO_SCHEDULE"
SUBNET_IDS=("subnet-xxxxxxxx" "subnet-xxxxxxxx") # Update with your subnet IDs

# Create the managed node group
aws eks create-nodegroup --cluster-name $CLUSTER_NAME \
    --nodegroup-name $NODE_GROUP_NAME \
    --disk-size 20 \
    --scaling-config minSize=$MIN_SIZE,maxSize=$MAX_SIZE,desiredSize=$DESIRED_SIZE \
    --instance-types $INSTANCE_TYPE \
    --ami-type $AMI_TYPE \
    --capacity-type $CAPACITY_TYPE \
    --node-role $ROLE_ARN \
    --labels $LABEL_KEY=$LABEL_VALUE \
    --taints key=$TAINT_KEY,value=$TAINT_VALUE,effect=$TAINT_EFFECT \
    --subnets $SUBNET_IDS

# The following commands are only needed right after node group creation to combat an odd quirk with Cluster Autoscaler
# where it fails to launch pods onto a new node in the node group if the desiredSize is 0 during the initial node group creation

# Update the node group's minSize and desiredSize to 1
aws eks update-nodegroup-config --cluster-name $CLUSTER_NAME --nodegroup-name $NODE_GROUP_NAME --scaling-config minSize=1,maxSize=$MAX_SIZE,desiredSize=1

# Update the node group's minSize and desiredSize back to 0
aws eks update-nodegroup-config --cluster-name $CLUSTER_NAME --nodegroup-name $NODE_GROUP_NAME --scaling-config minSize=0,maxSize=$MAX_SIZE,desiredSize=0