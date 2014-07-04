package aws

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform/flatmap"
	"github.com/hashicorp/terraform/helper/diff"
	"github.com/hashicorp/terraform/terraform"
	"github.com/mitchellh/goamz/elb"
)

func resource_aws_elb_create(
	s *terraform.ResourceState,
	d *terraform.ResourceDiff,
	meta interface{}) (*terraform.ResourceState, error) {
	p := meta.(*ResourceProvider)
	elbconn := p.elbconn

	// Merge the diff into the state so that we have all the attributes
	// properly.
	rs := s.MergeDiff(d)

	// The name specified for the ELB. This is also our unique ID
	// we save to state if the creation is succesful (amazon verifies
	// it is unique)
	elbName := rs.Attributes["name"]

	// Expand the "listener" array to goamz compat []elb.Listener
	v := flatmap.Expand(rs.Attributes, "listener").([]interface{})
	listeners := expandListeners(v)

	v = flatmap.Expand(rs.Attributes, "availability_zones").([]interface{})
	zones := expandStringList(v)

	// Provision the elb
	elbOpts := &elb.CreateLoadBalancer{
		LoadBalancerName: elbName,
		Listeners:        listeners,
		AvailZone:        zones,
	}

	log.Printf("[DEBUG] ELB create configuration: %#v", elbOpts)

	_, err := elbconn.CreateLoadBalancer(elbOpts)
	if err != nil {
		return nil, fmt.Errorf("Error creating ELB: %s", err)
	}

	// Assign the elb's unique identifer for use later
	rs.ID = elbName
	log.Printf("[INFO] ELB ID: %s", elbName)

	// If we have any instances, we need to register them
	v = flatmap.Expand(rs.Attributes, "instances").([]interface{})
	instances := expandStringList(v)

	if len(instances) > 0 {
		registerInstancesOpts := elb.RegisterInstancesWithLoadBalancer{
			LoadBalancerName: elbName,
			Instances:        instances,
		}

		_, err := elbconn.RegisterInstancesWithLoadBalancer(&registerInstancesOpts)

		if err != nil {
			return nil, fmt.Errorf("Failure registering instances: %s", err)
		}
	}

	describeElbOpts := &elb.DescribeLoadBalancer{
		Names: []string{elbName},
	}

	// Retrieve the ELB properties for updating the state
	describeResp, err := elbconn.DescribeLoadBalancers(describeElbOpts)
	if err != nil {
		return nil, fmt.Errorf("Error retrieving ELB: %s", err)
	}

	loadBalancer := describeResp.LoadBalancers[0]

	// Verify AWS returned our ELB
	if len(describeResp.LoadBalancers) != 1 ||
		describeResp.LoadBalancers[0].LoadBalancerName != elbName {
		if err != nil {
			return nil, fmt.Errorf("Unable to find ELB: %#v", describeResp.LoadBalancers)
		}
	}

	return resource_aws_elb_update_state(rs, &loadBalancer)
}

func resource_aws_elb_update(
	s *terraform.ResourceState,
	d *terraform.ResourceDiff,
	meta interface{}) (*terraform.ResourceState, error) {
	// p := meta.(*ResourceProvider)
	// elbconn := p.elbconn

	// Merge the diff into the state so that we have all the attributes
	// properly.
	// rs := s.MergeDiff(d)

	return nil, nil
}

func resource_aws_elb_destroy(
	s *terraform.ResourceState,
	meta interface{}) error {

	return nil
}

func resource_aws_elb_refresh(
	s *terraform.ResourceState,
	meta interface{}) (*terraform.ResourceState, error) {

	loadBalancer := &elb.LoadBalancer{}

	return resource_aws_elb_update_state(s, loadBalancer)
}

func resource_aws_elb_diff(
	s *terraform.ResourceState,
	c *terraform.ResourceConfig,
	meta interface{}) (*terraform.ResourceDiff, error) {

	b := &diff.ResourceBuilder{
		Attrs: map[string]diff.AttrType{
			"name":              diff.AttrTypeCreate,
			"availability_zone": diff.AttrTypeCreate,
			"listener":          diff.AttrTypeCreate,
			"instances":         diff.AttrTypeUpdate,
		},

		ComputedAttrs: []string{
			"dns_name",
		},
	}

	return b.Diff(s, c)
}

func resource_aws_elb_update_state(
	s *terraform.ResourceState,
	balancer *elb.LoadBalancer) (*terraform.ResourceState, error) {
	s.Attributes["name"] = balancer.LoadBalancerName
	s.Attributes["dns_name"] = balancer.DNSName
	return s, nil
}
