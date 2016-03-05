function PatchController($scope, $filter, $window, notificationService) {
  $scope.userTz = $window.userTz;
  $scope.canEdit = $window.canEdit;

  $scope.ALL_VARIANTS_KEY = 'ALL'

  $scope.selectedVariant = {
    v: "",
    get: function() {
      return $scope.selectedVariant.v;
    },
    set: function(val) {
      $scope.selectedVariant.v = val;
    }
  };

  $scope.xxx = function(){
    console.log(arguments)
  }
  
  $scope.numSetForVariant = function(variantId){
    return _.values($scope.selectedTasksByVariant[variantId]).filter(function(x){return x}).length || undefined
  }


  $scope.setPatchInfo = function() {
    $scope.patch = $window.patch;
    $scope.patchContainer = {'Patch':$scope.patch}
    var patch = $scope.patch;

    var allTasks = _.sortBy($window.tasks, 'Name')
    var allVariants = $window.variants;

    var selectedTasksByVariant = {}
    console.log(selectedTasksByVariant)

    var allVariantsModels = [];
    var allVariantsModelsOriginal = [];
    for (var variantId in allVariants) {
      var variant = {
        "name": allVariants[variantId].DisplayName,
        "id": variantId,
        "tasks": _.map(allVariants[variantId].Tasks, function(task) {
          return task.Name;
        })
      };
      if ($.inArray(variant.id, patch.BuildVariants) >= 0)  {
        variant.checked = true;
      }
      allVariantsModels.push(variant);
      allVariantsModelsOriginal.push(_.clone(variant));
    }

    _.each(allVariantsModelsOriginal, function(x){
      selectedTasksByVariant[x.id] = {}
      _.each(allTasks, function(y){
        selectedTasksByVariant[x.id][y.Name] = false
      })
    })

    // populate a special "all variants" variant
    // that has all the tasks under key ''.
    selectedTasksByVariant[''] = {};
    allTasks.forEach(function(x){
      selectedTasksByVariant[''][x.Name] = false
    });

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

    // Whenever the user makes changes to the "all variants" variant,
    // go back and update the selections on the normal variants.
    /*
    $scope.$watch(
      function($scope){
        return $scope.selectedTasksByVariant['']
      },
      function(oldVal, newVal){
        _.each($scope.selectedTasksByVariant[''], function(allVariantTask, activated){
          _.each($scope.selectedTasksByVariant, function(variant, tasks){
            if(variant != '' && $scope.taskRunsOnVariant(allVariantTask, variant)){
              $scope.
            }
          }
        })
      },
      true // does a deep watch on the array.
    )
    */

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
    $scope.allVariants = allVariantsModels;
    $scope.allVariantsOriginal = allVariantsModelsOriginal;

    $scope.$watch('allVariants', function(allVariants) {
      $scope.variantsCount = 0;
      _.forEach(allVariants, function(item) {
        if (item.checked) {
          $scope.variantsCount += 1;
        }
      });
    }, true);

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

  $scope.selectedVariants = function() {
    return $filter('filter')($scope.allVariants, {
      checked: true
    });
  };

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
}

function PatchUpdateController($scope, $http) {
  $scope.scheduleBuilds = function(form) {
    var data = {
      variants: [],
      tasks: [],
      description: $scope.patch.Description
    };
    var selectedVariants = $scope.selectedVariants();
    var selectedTasks = $scope.selectedTasks();

    for (var i = 0; i < selectedVariants.length; ++i) {
      data.variants.push(selectedVariants[i].id);
    }

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
