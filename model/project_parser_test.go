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
	})
}
