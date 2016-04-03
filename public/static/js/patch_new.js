mciModule.controller('PatchController', function($scope, $filter, $window, notificationService) {
  $scope.userTz = $window.userTz;
  $scope.canEdit = $window.canEdit;

  $scope.selectVariant = function($event, index){
    $event.preventDefault()
    if ($event.ctrlKey || $event.metaKey) {
      // Ctrl/Meta+Click: Toggle just the variant being clicked.
      $scope.allVariants[index].checked = !$scope.allVariants[index].checked
    } else if ($event.shiftKey) {
      // Shift+Click: Select everything between the first element 
      // that's already selected element and the element being clicked on.
      var firstCheckedIndex = _.findIndex($scope.allVariants, function(x){ return x.checked })
      firstCheckedIndex = Math.max(firstCheckedIndex, 0) // if nothing selected yet, start at 0.
      var indexBounds = Array(firstCheckedIndex, index).sort(function(a, b){
        return a-b;
      })
      for(var i=indexBounds[0]; i<=indexBounds[1]; i++){
        $scope.allVariants[i].checked = true
      }
    } else {
      // Regular click: Select *only* the one being clicked, and unselect all others.
      for(var i=0; i<$scope.allVariants.length;i++){
        $scope.allVariants[i].checked = (i == index)
      }
    }
  }

  $scope.getActiveTasks = function(variants){
    // look at the set of variants that are chosen
    var selectedVariants = _.filter(variants, function(x){ return x.checked })

    // return the union of the set of tasks shared by all of them, sorted by name
    var tasksInSelectedVariants = _.uniq(_.flatten(_.pluck(selectedVariants, "tasks")))
    return tasksInSelectedVariants.sort()
  }

  $scope.numSetForVariant = function(variantId){
    return _.values($scope.selectedTasksByVariant[variantId]).filter(function(x){return x}).length || undefined
  }


  $scope.setPatchInfo = function() {
    $scope.patch = $window.patch;
    $scope.patchContainer = {'Patch':$scope.patch}
    var patch = $scope.patch;

    var rawVariants = $window.variants
    var allTasks = _.sortBy($window.tasks, 'Name')
    var allVariants = [];

    var selectedTasksByVariant = {}

    var allVariantsModels = [];
    var allVariantsModelsOriginal = [];
    for (var variantId in rawVariants) {
      var variant = {
        "name": rawVariants[variantId].DisplayName,
        "id": variantId,
        "checked": false,
        "tasks": _.map(rawVariants[variantId].Tasks, function(task) {
          return task.Name;
        })
      };
      allVariants.push(variant)
    }

    $scope.selectedTasksByVariant = selectedTasksByVariant
    console.log(selectedTasksByVariant)

    // Create a map from tasks to list of build variants that run that task
    $scope.buildVariantsForTask = {};
    _.each(allVariantsModelsOriginal, function(variant) {
      _.each(variant.tasks, function(task) {
        $scope.buildVariantsForTask[task] =
          $scope.buildVariantsForTask[task] || [];
        if (variant.id &&
          $scope.buildVariantsForTask[task].indexOf(variant.id) == -1) {
          $scope.buildVariantsForTask[task].push(variant.id);
        }
      });
    });

    var allTasksModels = [];
    var allTasksModelsOriginal = [];
    for (var i = 0; i < allTasks.length; ++i) {
      task = allTasks[i];
      if (task.Name === "compile" || $.inArray(task.Name, patch.Tasks) >= 0) {
        task.checked = true;
      }
      allTasksModels.push(task);
      allTasksModelsOriginal.push(_.clone(task));
    }
    $scope.allTasks = allTasksModels;
    $scope.allTasksOriginal = allTasksModelsOriginal;
    $scope.allVariants = allVariants;
    $scope.allVariantsOriginal = allVariantsModelsOriginal;

  /*
    $scope.$watch('allVariants', function(allVariants) {
      $scope.variantsCount = 0;
      _.forEach(allVariants, function(item) {
        if (item.checked) {
          $scope.variantsCount += 1;
        }
      });
    }, true);
    */

    $scope.$watch('allTasks', function(allTasks) {
      $scope.taskCount = 0;
      _.forEach(allTasks, function(item) {
        if (item.checked) {
          $scope.taskCount += 1;
        }
      });
    }, true);
  };
  $scope.setPatchInfo();

  $scope.getAllVariantsVariant = function(){
    return $scope.selectedTasksByVariant['']
  }

  $scope.selectedTasks = function() {
    return $filter('filter')($scope.allTasks, {
      checked: true
    });
  };

  $scope.toggleCheck = function(x) {
    x.checked = !x.checked;
  };

  $scope.variantRunsTask = function(variant, taskName) {
    // Does this variant run the given task name?
    return variant.tasks.indexOf(taskName) != -1;
  };

  $scope.taskRunsOnVariant = function(taskName, variant) {
    if(variant == ''){  // used for the "all variants" pseudo-variant.
      return true
    }
    // Does this task name run on the variant with the given name?
    return ($scope.buildVariantsForTask[taskName] || []).indexOf(variant) != -1;
  };
})

function PatchUpdateController($scope, $http) {
  $scope.scheduleBuilds = function(form) {
    var data = {
      variants: [],
      tasks: [],
      description: $scope.patch.Description
    };
    var selectedTasks = $scope.selectedTasks();

    for (var i = 0; i < selectedTasks.length; ++i) {
      data.tasks.push(selectedTasks[i].Name);
    }

    $http.post('/patch/' + $scope.patch.Id, data).
    success(function(data, status) {
      window.location.replace("/version/" + data.version);
    }).
    error(function(data, status, errorThrown) {
      alert("Failed to save changes: `" + data + "`",'errorHeader');
    });
  };

  $scope.select = function(models, state) {
    for (var i = 0; i < models.length; ++i) {
      models[i].checked = state;
    }
  };
}
