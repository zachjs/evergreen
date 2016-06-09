package model

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCreateIntermediateProjectDependencies(t *testing.T) {
	Convey("Testing different project files", t, func() {
		pp := &projectParser{}
		Convey("a simple project file should parse", func() {
			simple := `
tasks:
- name: "compile"
- name: task0
- name: task1
  patchable: false
  tags: ["tag1", "tag2"]
  depends_on:
  - compile
  - name: "task0"
    status: "failed"
    patch_optional: true
`
			pass := pp.createIntermediateProject([]byte(simple))
			So(pass, ShouldBeTrue)
			So(len(pp.errors), ShouldEqual, 0)
			So(len(pp.warnings), ShouldEqual, 0)
			So(pp.p.Tasks[2].DependsOn[0].Name, ShouldEqual, "compile")
			So(pp.p.Tasks[2].DependsOn[0].PatchOptional, ShouldEqual, false)
			So(pp.p.Tasks[2].DependsOn[1].Name, ShouldEqual, "task0")
			So(pp.p.Tasks[2].DependsOn[1].Status, ShouldEqual, "failed")
			So(pp.p.Tasks[2].DependsOn[1].PatchOptional, ShouldEqual, true)
		})
		Convey("a file with a single dependency should parse", func() {
			single := `
tasks:
- name: "compile"
- name: task0
- name: task1
  depends_on: task0
`
			pass := pp.createIntermediateProject([]byte(single))
			So(pass, ShouldBeTrue)
			So(len(pp.errors), ShouldEqual, 0)
			So(len(pp.warnings), ShouldEqual, 0)
			So(pp.p.Tasks[2].DependsOn[0].Name, ShouldEqual, "task0")
		})
		Convey("a file with a nameless dependency should error", func() {
			Convey("with a single dep", func() {
				nameless := `
tasks:
- name: "compile"
  depends_on: ""
`
				pass := pp.createIntermediateProject([]byte(nameless))
				So(pass, ShouldBeFalse)
				So(len(pp.errors), ShouldEqual, 1)
				So(len(pp.warnings), ShouldEqual, 0)
			})
			Convey("or multiple", func() {
				nameless := `
tasks:
- name: "compile"
  depends_on:
  - name: "task1"
  - status: "failed" #this has no task attached
`
				pass := pp.createIntermediateProject([]byte(nameless))
				So(pass, ShouldBeFalse)
				So(len(pp.errors), ShouldEqual, 1)
				So(len(pp.warnings), ShouldEqual, 0)
			})
		})
	})
}

func TestCreateIntermediateProjectRequirements(t *testing.T) {
	Convey("Testing different project files", t, func() {
		pp := &projectParser{}
		Convey("a simple project file should parse", func() {
			simple := `
tasks:
- name: task0
- name: task1
  requires:
  - name: "task0"
    variant: "v1"
  - "task2"
`
			pass := pp.createIntermediateProject([]byte(simple))
			So(pass, ShouldBeTrue)
			So(len(pp.errors), ShouldEqual, 0)
			So(len(pp.warnings), ShouldEqual, 0)
			So(pp.p.Tasks[1].Requires[0].Name, ShouldEqual, "task0")
			So(pp.p.Tasks[1].Requires[0].Variant, ShouldEqual, "v1")
			So(pp.p.Tasks[1].Requires[1].Name, ShouldEqual, "task2")
			So(pp.p.Tasks[1].Requires[1].Variant, ShouldEqual, "")
		})
		Convey("a single requirement should parse", func() {
			simple := `
tasks:
- name: task1
  requires:
    name: "task0"
    variant: "v1"
`
			pass := pp.createIntermediateProject([]byte(simple))
			So(pass, ShouldBeTrue)
			So(len(pp.errors), ShouldEqual, 0)
			So(len(pp.warnings), ShouldEqual, 0)
			So(pp.p.Tasks[0].Requires[0].Name, ShouldEqual, "task0")
			So(pp.p.Tasks[0].Requires[0].Variant, ShouldEqual, "v1")
		})
	})
}

func TestCreateIntermediateProjectBuildVariants(t *testing.T) {
	Convey("Testing different project files", t, func() {
		pp := &projectParser{}
		Convey("a file with multiple BVTs should parse", func() {
			simple := `
buildvariants:
- name: "v1"
  stepback: true
  batchtime: 123
  modules: ["wow","cool"]
  run_on:
  - "windows2000"
  tasks:
  - name: "t1"
  - name: "t2"
    depends_on:
    - name: "t3"
      variant: "v0"
    requires:
    - name: "t4"
    stepback: false
    priority: 77
`
			pass := pp.createIntermediateProject([]byte(simple))
			So(pass, ShouldBeTrue)
			So(len(pp.errors), ShouldEqual, 0)
			So(len(pp.warnings), ShouldEqual, 0)
			bv := pp.p.BuildVariants[0]
			So(bv.Name, ShouldEqual, "v1")
			So(*bv.Stepback, ShouldBeTrue)
			So(bv.RunOn[0], ShouldEqual, "windows2000")
			So(len(bv.Modules), ShouldEqual, 2)
			So(bv.Tasks[0].Name, ShouldEqual, "t1")
			So(bv.Tasks[1].Name, ShouldEqual, "t2")
			So(bv.Tasks[1].DependsOn[0].TaskSelector, ShouldResemble, TaskSelector{Name: "t3", Variant: "v0"})
			So(bv.Tasks[1].Requires[0], ShouldResemble, TaskSelector{Name: "t4"})
			So(*bv.Tasks[1].Stepback, ShouldBeFalse)
			So(bv.Tasks[1].Priority, ShouldEqual, 77)
		})
		Convey("a file with oneline BVTs should parse", func() {
			simple := `
buildvariants:
- name: "v1"
  tasks:
  - "t1"
  - name: "t2"
    depends_on: "t3"
    requires: "t4"
`
			pass := pp.createIntermediateProject([]byte(simple))
			So(pass, ShouldBeTrue)
			So(len(pp.errors), ShouldEqual, 0)
			So(len(pp.warnings), ShouldEqual, 0)
			bv := pp.p.BuildVariants[0]
			So(bv.Name, ShouldEqual, "v1")
			So(bv.Tasks[0].Name, ShouldEqual, "t1")
			So(bv.Tasks[1].Name, ShouldEqual, "t2")
			So(bv.Tasks[1].DependsOn[0].TaskSelector, ShouldResemble, TaskSelector{Name: "t3"})
			So(bv.Tasks[1].Requires[0], ShouldResemble, TaskSelector{Name: "t4"})
		})
		Convey("a file with single BVTs should parse", func() {
			simple := `
buildvariants:
- name: "v1"
  tasks: "*"
- name: "v2"
  tasks:
    name: "t1"
`
			pass := pp.createIntermediateProject([]byte(simple))
			So(pass, ShouldBeTrue)
			So(len(pp.errors), ShouldEqual, 0)
			So(len(pp.warnings), ShouldEqual, 0)
			So(len(pp.p.BuildVariants), ShouldEqual, 2)
			bv1 := pp.p.BuildVariants[0]
			bv2 := pp.p.BuildVariants[1]
			So(bv1.Name, ShouldEqual, "v1")
			So(bv2.Name, ShouldEqual, "v2")
			So(len(bv1.Tasks), ShouldEqual, 1)
			So(bv1.Tasks[0].Name, ShouldEqual, "*")
			So(len(bv2.Tasks), ShouldEqual, 1)
			So(bv2.Tasks[0].Name, ShouldEqual, "t1")
		})
	})
}

func TestTranslateDependsOn(t *testing.T) {
	Convey("With an intermediate parseProject", t, func() {
		pp := &projectParser{p: &parserProject{}}
		Convey("a tag-free dependency config should be unchanged", func() {
			pp.p.Tasks = []parserTask{
				{Name: "t1"},
				{Name: "t2"},
				{Name: "t3", DependsOn: parserDependencies{
					{TaskSelector: TaskSelector{Name: "t1"}},
					{TaskSelector: TaskSelector{Name: "t2", Variant: "v1"}}},
				},
			}
			out := pp.translateProject()
			So(pp.errors, ShouldBeNil)
			So(pp.warnings, ShouldBeNil)
			So(out, ShouldNotBeNil)
			deps := out.Tasks[2].DependsOn
			So(deps[0].Name, ShouldEqual, "t1")
			So(deps[1].Name, ShouldEqual, "t2")
			So(deps[1].Variant, ShouldEqual, "v1")
		})
		Convey("a dependency with tag selectors should evaluate", func() {
			pp.p.Tasks = []parserTask{
				{Name: "t1", Tags: []string{"a", "b"}},
				{Name: "t2", Tags: []string{"a", "c"}},
				{Name: "t3", DependsOn: parserDependencies{
					{TaskSelector: TaskSelector{Name: ".b"}},
					{TaskSelector: TaskSelector{Name: ".a !.b", Variant: "v1"}}},
				},
			}
			out := pp.translateProject()
			So(pp.errors, ShouldBeNil)
			So(pp.warnings, ShouldBeNil)
			So(out, ShouldNotBeNil)
			deps := out.Tasks[2].DependsOn
			So(deps[0].Name, ShouldEqual, "t1")
			So(deps[1].Name, ShouldEqual, "t2")
			So(deps[1].Variant, ShouldEqual, "v1")
		})
		Convey("a dependency with erroneous selectors should fail", func() {
			pp.p.Tasks = []parserTask{
				{Name: "t1", Tags: []string{"a", "b"}},
				{Name: "t2", Tags: []string{"a", "c"}},
				{Name: "t3", DependsOn: parserDependencies{
					{TaskSelector: TaskSelector{Name: ".cool"}},
					{TaskSelector: TaskSelector{Name: "!!.cool"}},                //illegal selector
					{TaskSelector: TaskSelector{Name: "!.c !.b", Variant: "v1"}}, //no matching tasks
					{TaskSelector: TaskSelector{Name: "t1"}, Status: "*"},        //valid, but:
					{TaskSelector: TaskSelector{Name: ".b"}},                     //conflicts with above
				}},
			}
			out := pp.translateProject()
			So(len(pp.errors), ShouldEqual, 3)
			So(pp.warnings, ShouldBeNil)
			So(out, ShouldNotBeNil)
		})
	})
}

func TestTranslateRequires(t *testing.T) {
	Convey("With an intermediate parseProject", t, func() {
		pp := &projectParser{p: &parserProject{}}
		Convey("a task with valid requirements should succeed", func() {
			pp.p.Tasks = []parserTask{
				{Name: "t1"},
				{Name: "t2"},
				{Name: "t3", Requires: TaskSelectors{
					{Name: "t1"},
					{Name: "t2", Variant: "v1"},
				}},
			}
			out := pp.translateProject()
			So(pp.errors, ShouldBeNil)
			So(pp.warnings, ShouldBeNil)
			So(out, ShouldNotBeNil)
			reqs := out.Tasks[2].Requires
			So(reqs[0].Name, ShouldEqual, "t1")
			So(reqs[1].Name, ShouldEqual, "t2")
			So(reqs[1].Variant, ShouldEqual, "v1")
		})
		Convey("a task with erroneous requirements should fail", func() {
			pp.p.Tasks = []parserTask{
				{Name: "t1"},
				{Name: "t2", Tags: []string{"taggy"}},
				{Name: "t3", Requires: TaskSelectors{
					{Name: "!!!!!"},                     //illegal selector
					{Name: ".taggy !t2", Variant: "v1"}, //nothing returned
					{Name: "t1 t2"},                     //nothing returned
				}},
			}
			out := pp.translateProject()
			So(len(pp.errors), ShouldEqual, 3)
			So(pp.warnings, ShouldBeNil)
			So(out, ShouldNotBeNil)
		})
	})
}
