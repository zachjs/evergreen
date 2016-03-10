var directives = directives || {};

directives.tristateCheckbox = angular.module('directives.tristateCheckbox', []);
directives.tristateCheckbox.directive('tristateCheckbox', [function() {
   return {
     scope: true,
     require: '?ngModel',
     link: function(scope, element, attrs, modelCtrl) {
        //console.log("hi please i am in here", scope, element, attrs, modelCtrl)
        console.log(attrs.indeterminate)
        if(attrs.indeterminate){
          element[0].indeterminate = true
        }

      /*
       element.bind('change', function() {
         console.log("changed!")
         console.log(element, element[0])
         element.indeterminate = true
         element[0].indeterminate = true
         scope.$apply()
       })
       */

/*
       var childList = attrs.childList;
       var property = attrs.property;
            
       // Bind the onChange event to update children
       element.bind('change', function() {
                scope.$apply(function () {
                    var isChecked = element.prop('checked');
                    
                    // Set each child's selected property to the checkbox's checked property
                    angular.forEach(scope.$eval(childList), function(child) {
                        child[property] = isChecked;
                    });
                });
            });
            
            // Watch the children for changes
            scope.$watch(childList, function(newValue) {
                var hasChecked = false;
                var hasUnchecked = false;
                
                // Loop through the children
                angular.forEach(newValue, function(child) {
                    if (child[property]) {
                        hasChecked = true;
                    } else {
                        hasUnchecked = true;
                    }
                });
                
                // Determine which state to put the checkbox in
                if (hasChecked && hasUnchecked) {
                    element.prop('checked', false);
                    element.prop('indeterminate', true);
                    if (modelCtrl) {
                        modelCtrl.$setViewValue(false);
                    }
                } else {
                    element.prop('checked', hasChecked);
                    element.prop('indeterminate', false);
                    if (modelCtrl) {
                        modelCtrl.$setViewValue(hasChecked);
                    }
                }
            }, true);
*/
        }
    };
}]);

