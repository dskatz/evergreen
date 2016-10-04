 /*
  ReactJS code for the Waterfall page. Grid calls the Variant class for each distro, and the Variant class renders each build variant for every version that exists. In each build variant we iterate through all the tasks and render them as well. The row of headers is just a placeholder at the moment.
 */

const MaxFailedTestDisplay = 5;
    

// Returns string from datetime object in "5/7/96 1:15 AM" format
// Used to display version headers
function getFormattedTime(input, userTz, fmt) {
  return moment(input).tz(userTz).format(fmt);
}


// taskStatusClass returns the css class that should be associated with a given task so that it can
// be properly styled.
function taskStatusClass(task) {
  if (task !== Object(task)) {
	  return '';
  }

  if (task.status == 'undispatched') {
    if (!task.activated) {
      return 'inactive';
    } else {
      return 'unstarted';
    }
  }

  if (task.status == 'failed') {
    if ('task_end_details' in task) {
      if ('type' in task.task_end_details && task.task_end_details.type == 'system') {
         return 'system-failed';
      }
      if (!!task.task_end_details.timed_out && task.task_end_details.desc == 'heartbeat') {
        return 'system-failed';
      }
    }
    return 'failed';
  }
  return task.status;
}

// endOfPath strips off all of the begging characters from a file path so that just the file name is left.
function endOfPath(input) {
  var lastSlash = input.lastIndexOf('/');
  if (lastSlash === -1 || lastSlash === input.length - 1) {
    // try to find the index using windows-style filesystem separators
    lastSlash = input.lastIndexOf('\\');
    if (lastSlash === -1 || lastSlash === input.length - 1) {
      return input;
    }
  }
  return input.substring(lastSlash + 1);
}

function generateURLParameters(params) {
 var ret = [];
 for (var p in params) {
  ret.push(encodeURIComponent(p) + "=" + encodeURIComponent(params[p]));
 }
 return ret.join("&");
}

// getParameterByName returns the value associated with a given query parameter.
// Based on: http://stackoverflow.com/questions/901115/how-can-i-get-query-string-values-in-javascript
function getParameterByName(name, url) {
  name = name.replace(/[\[\]]/g, "\\$&");
  var regex = new RegExp("[?&]" + name + "(=([^&#]*)|&|#|$)");
  var results = regex.exec(url);
  if (!results){
    return null;
  }
  if (!results[2]){
    return '';
  }
  return decodeURIComponent(results[2].replace(/\+/g, " "));
}

function updateURLParams(bvFilter, taskFilter, skip, baseURL) {
  var params = {};
  if (bvFilter && bvFilter != '')
    params["bv_filter"]= bvFilter;
  if (taskFilter && taskFilter != '')
    params["task_filter"]= taskFilter; 
  params["skip"] = skip

  var paramString = generateURLParameters(params);
  window.history.replaceState({}, '', baseURL + "?" + paramString);
}

// labelFromTask returns the human readable label for a task's status given the details of its execution.
function labelFromTask(task){
  if (task !== Object(task)) {
	  return '';
  }

  if (task.status == 'undispatched') {
    if (task.activated) {
      if (task.task_waiting) {
        return task.task_waiting;
      }
      return 'scheduled';
    } else if (+task.dispatch_time == 0 || (typeof task.dispatch_time == "string" && +new Date(task.dispatch_time) <= 0)) {
       return 'not scheduled';
    }
  }

  if (task.status == 'failed' && 'task_end_details' in task){
    if ('timed_out' in task.task_end_details) {
      if (task.task_end_details.timed_out && task.task_end_details.desc == 'heartbeat') {
        return 'system unresponsive';
      }
      if (task.task_end_details.type == 'system') {
        return 'system timed out';
      }
      return 'test timed out';
    }
    if (task.task_end_details.type == 'system') {
      return 'system failure';
    }
  }

  return task.status;
}


// stringifyNanoseconds takes an integer count of nanoseconds and
// returns it formatted as a human readable string, like "1h32m40s"
// If skipDayMax is true, then durations longer than 1 day will be represented
// in hours. Otherwise, they will be displayed as '>=1 day'
function stringifyNanoseconds(input, skipDayMax, skipSecMax) {
  var NS_PER_MS = 1000 * 1000; // 10^6
  var NS_PER_SEC = NS_PER_MS * 1000
  var NS_PER_MINUTE = NS_PER_SEC * 60;
  var NS_PER_HOUR = NS_PER_MINUTE * 60;

  if (input == 0) {
    return "0 seconds";
  } else if (input < NS_PER_MS) {
    return "< 1 ms";
  } else if (input < NS_PER_SEC) {
    if (skipSecMax){
      return Math.floor(input / NS_PER_MS) + " ms";
    } else {
      return "< 1 second"
    }
  } else if (input < NS_PER_MINUTE) {
    return Math.floor(input / NS_PER_SEC) + " seconds";
  } else if (input < NS_PER_HOUR) {
    return Math.floor(input / NS_PER_MINUTE) + "m " + Math.floor((input % NS_PER_MINUTE) / NS_PER_SEC) + "s";
  } else if (input < NS_PER_HOUR * 24 || skipDayMax) {
    return Math.floor(input / NS_PER_HOUR) + "h " +
        Math.floor((input % NS_PER_HOUR) / NS_PER_MINUTE) + "m " +
        Math.floor((input % NS_PER_MINUTE) / NS_PER_SEC) + "s";
  } else if (input == "unknown") {
    return "unknown";
  }  else {
    return ">= 1 day";
  }
}


// The Root class renders all components on the waterfall page, including the grid view and the filter and new page buttons
// The one exception is the header, which is written in Angular and managed by menu.html
class Root extends React.Component{
  constructor(props){
    super(props);
    // Initialize newer|older buttons
    var versionsOnPage = _.reduce(_.map(window.serverData.versions, function(version){
      return version.authors.length; 
    }), function(memo,num){
      return memo + num;
    });

    this.baseURL = "/waterfall/" + this.props.project;
    this.currentSkip = window.serverData.current_skip;
    this.nextSkip = this.currentSkip + versionsOnPage; 
    this.prevSkip = this.currentSkip - window.serverData.previous_page_count;

    if (this.nextSkip >= window.serverData.total_versions) {
      this.nextSkip = -1;
    }
    if (this.currentSkip <= 0) {
      this.prevSkip = -1;
    }
   
    var buildVariantFilter = getParameterByName('bv_filter',window.location.href);
    var taskFilter = getParameterByName('task_filter',window.location.href);

    buildVariantFilter = buildVariantFilter || '';
    taskFilter = taskFilter || '';
    
    var collapsed = localStorage.getItem("collapsed") == "true";

    this.state = {
      collapsed: collapsed,
      shortenCommitMessage: true,
      buildVariantFilter: buildVariantFilter,
      taskFilter:taskFilter 
    };

    // Handle state for a collapsed view, as well as shortened header commit messages
    this.handleCollapseChange = this.handleCollapseChange.bind(this);
    this.handleHeaderLinkClick = this.handleHeaderLinkClick.bind(this);
    this.handleBuildVariantFilter = this.handleBuildVariantFilter.bind(this);
    this.handleTaskFilter = this.handleTaskFilter.bind(this);
  }

  handleCollapseChange(collapsed) {
    localStorage.setItem("collapsed", collapsed);
    this.setState({collapsed: collapsed});
  }
  handleBuildVariantFilter(filter) {
    updateURLParams(filter, this.state.taskFilter, this.currentSkip, this.baseURL);
    this.setState({buildVariantFilter: filter});
  }
  handleTaskFilter(filter) {
    updateURLParams(this.state.buildVariantFilter, filter, this.currentSkip, this.baseURL);
    this.setState({taskFilter: filter});
  }
  handleHeaderLinkClick(shortenMessage) {
    this.setState({shortenCommitMessage: !shortenMessage});
  }
  render() {
    if (this.props.data.rows.length == 0){
      return (
        React.createElement("div", null, 
          "There are no builds for this project."
        )
        )
    }
    var collapseInfo = {
      collapsed : this.state.collapsed,
      activeTaskStatuses : ['failed','system-failed'],
    };
    return (
      React.createElement("div", null, 
        React.createElement(Toolbar, {
          collapsed: this.state.collapsed, 
          onCheck: this.handleCollapseChange, 
          baseURL: this.baseURL, 
          nextSkip: this.nextSkip, 
          prevSkip: this.prevSkip, 
          buildVariantFilter: this.state.buildVariantFilter, 
          taskFilter: this.state.taskFilter, 
          buildVariantFilterFunc: this.handleBuildVariantFilter, 
          taskFilterFunc: this.handleTaskFilter}
        ), 
        React.createElement(Headers, {
          shortenCommitMessage: this.state.shortenCommitMessage, 
          versions: this.props.data.versions, 
          onLinkClick: this.handleHeaderLinkClick, 
          userTz: this.props.userTz}
        ), 
        React.createElement(Grid, {
          data: this.props.data, 
          collapseInfo: collapseInfo, 
          project: this.props.project, 
          buildVariantFilter: this.state.buildVariantFilter, 
          taskFilter: this.state.taskFilter}
        )
      )
    )
  }
}


// Toolbar
function Toolbar ({collapsed, 
  onCheck, 
  baseURL,
  nextSkip, 
  prevSkip, 
  buildVariantFilter, 
  taskFilter,
  buildVariantFilterFunc, 
  taskFilterFunc}) {

  var Form = ReactBootstrap.Form;
  return (
    React.createElement("div", {className: "row"}, 
      React.createElement("div", {className: "col-xs-12"}, 
        React.createElement(Form, {inline: true, className: "waterfall-toolbar pull-right"}, 
          React.createElement(CollapseButton, {collapsed: collapsed, onCheck: onCheck}), 
          React.createElement(FilterBox, {
            filterFunction: buildVariantFilterFunc, 
            placeholder: "Filter variant", 
            currentFilter: buildVariantFilter, 
            disabled: false}
          ), 
          React.createElement(FilterBox, {
            filterFunction: taskFilterFunc, 
            placeholder: "Filter task", 
            currentFilter: taskFilter, 
            disabled: collapsed}
          ), 
          React.createElement(PageButtons, {
            nextSkip: nextSkip, 
            prevSkip: prevSkip, 
            baseURL: baseURL, 
            buildVariantFilter: buildVariantFilter, 
            taskFilter: taskFilter}
          )
        )
      )
    )
  )
};

function PageButtons ({prevSkip, nextSkip, baseURL, buildVariantFilter, taskFilter}) {
  var ButtonGroup = ReactBootstrap.ButtonGroup;

  var nextURL= "";
  var prevURL= "";

  prevURLParams = {};
  nextURLParams = {};

  nextURLParams["skip"] = nextSkip;
  prevURLParams["skip"] = prevSkip;
  if (buildVariantFilter && buildVariantFilter != '') {
    nextURLParams["bv_filter"] = buildVariantFilter;
    nextURLParams["bv_filter"] = buildVariantFilter;
  }
  if (taskFilter && taskFilter != '') {
    prevURLParams["task_filter"] = taskFilter;
    prevURLParams["task_filter"] = taskFilter;
  }
  nextURL = "?" + generateURLParameters(nextURLParams);
  prevURL = "?" + generateURLParameters(prevURLParams);
  return (
    React.createElement("span", {className: "waterfall-form-item"}, 
      React.createElement(ButtonGroup, null, 
        React.createElement(PageButton, {pageURL: prevURL, disabled: prevSkip < 0, directionIcon: "fa-chevron-left"}), 
        React.createElement(PageButton, {pageURL: nextURL, disabled: nextSkip < 0, directionIcon: "fa-chevron-right"})
      )
    )
  );
}

function PageButton ({pageURL, directionIcon, disabled}) {
  var Button = ReactBootstrap.Button;
  var classes = "fa " + directionIcon;
  return (
    React.createElement(Button, {href: pageURL, disabled: disabled}, React.createElement("i", {className: classes}))
  );
}

class FilterBox extends React.Component {
  constructor(props){
    super(props);
    this.applyFilter = this.applyFilter.bind(this);
  }
  applyFilter() {
    this.props.filterFunction(this.refs.searchInput.value)
  }
  render() {
    return React.createElement("input", {type: "text", ref: "searchInput", 
                  className: "form-control waterfall-form-item", 
                  placeholder: this.props.placeholder, 
                  value: this.props.currentFilter, onChange: this.applyFilter, 
                  disabled: this.props.disabled})
  }
}

class CollapseButton extends React.Component{
  constructor(props){
    super(props);
    this.handleChange = this.handleChange.bind(this);
  }
  handleChange(event){
    this.props.onCheck(this.refs.collapsedBuilds.checked);
  }
  render() {
    return (
      React.createElement("span", {className: "semi-muted waterfall-form-item"}, 
        React.createElement("span", {id: "collapsed-prompt"}, "Show collapsed view"), 
        React.createElement("input", {
          className: "checkbox waterfall-checkbox", 
          type: "checkbox", 
          checked: this.props.collapsed, 
          ref: "collapsedBuilds", 
          onChange: this.handleChange}
        )
      )
    )
  }
}

// Headers

function Headers ({shortenCommitMessage, versions, onLinkClick, userTz}) {
  return (
    React.createElement("div", {className: "row version-header"}, 
      React.createElement("div", {className: "variant-col col-xs-2 version-header-rolled"}), 
      React.createElement("div", {className: "col-xs-10"}, 
        React.createElement("div", {className: "row"}, 
        
          versions.map(function(version){
            if (version.rolled_up) {
              return React.createElement(RolledUpVersionHeader, {key: version.ids[0], version: version, userTz: userTz});
            }
            // Unrolled up version, no popover
            return (
              React.createElement(ActiveVersionHeader, {
                key: version.ids[0], 
                version: version, 
                userTz: userTz, 
                shortenCommitMessage: shortenCommitMessage, 
                onLinkClick: onLinkClick}
              )
            );
          })
        
        )
      )
    )
  )
}


function ActiveVersionHeader({shortenCommitMessage, version, onLinkClick, userTz}) {
  var message = version.messages[0];
  var author = version.authors[0];
  var id_link = "/version/" + version.ids[0];
  var commit = version.revisions[0].substring(0,5);
  var message = version.messages[0]; 
  var formatted_time = getFormattedTime(version.create_times[0], userTz, 'M/D/YY h:mm A' );
  const maxChars = 44 
  var button;
  if (message.length > maxChars) {
    // If we shorten the commit message, only display the first maxChars chars
    if (shortenCommitMessage) {
      message = message.substring(0, maxChars-3) + "...";
    }
    button = (
      React.createElement(HideHeaderButton, {onLinkClick: onLinkClick, shortenCommitMessage: shortenCommitMessage})
    );
  }
 
  return (
      React.createElement("div", {className: "header-col"}, 
        React.createElement("div", {className: "version-header-expanded"}, 
          React.createElement("div", {className: "col-xs-12"}, 
            React.createElement("div", {className: "row"}, 
              React.createElement("a", {className: "githash", href: id_link}, commit), 
              formatted_time
            )
          ), 
          React.createElement("div", {className: "col-xs-12"}, 
            React.createElement("div", {className: "row"}, 
              React.createElement("strong", null, author), " - ", message, 
              button
            )
          )
        )
      )
  )
};

class HideHeaderButton extends React.Component{
  constructor(props){
    super(props);
    this.onLinkClick = this.onLinkClick.bind(this);
  }
  onLinkClick(event){
    this.props.onLinkClick(this.props.shortenCommitMessage);
  }
  render() {
    var textToShow = this.props.shortenCommitMessage ? "more" : "less";
    return (
      React.createElement("span", {onClick: this.onLinkClick}, " ", React.createElement("a", {href: "#"}, textToShow), " ")
    )
  }
}

function RolledUpVersionHeader({version, userTz}){
  var Popover = ReactBootstrap.Popover;
  var OverlayTrigger = ReactBootstrap.OverlayTrigger;
  var Button = ReactBootstrap.Button;
  
  var versionStr = (version.messages.length > 1) ? "versions" : "version";
  var rolledHeader = version.messages.length + " inactive " + versionStr; 
 
  const popovers = (
    React.createElement(Popover, {id: "popover-positioned-bottom", title: ""}, 
      
        version.ids.map(function(id,i) {
          return React.createElement(RolledUpVersionSummary, {version: version, key: id, i: i, userTz: userTz})
        })
      
    )
  );

  return (
    React.createElement("div", {className: "header-col version-header-rolled"}, 
      React.createElement(OverlayTrigger, {trigger: "click", placement: "bottom", overlay: popovers, className: "col-xs-2"}, 
          React.createElement("span", {className: "pointer"}, " ", rolledHeader, " ")
      )
    )
  )
};

function RolledUpVersionSummary ({version, i, userTz}) {
  var formatted_time = getFormattedTime(new Date(version.create_times[i]), userTz, 'M/D/YY h:mm A' );
  var author = version.authors[i];
  var commit =  version.revisions[i].substring(0,10);
  var message = version.messages[i];
    
  return (
    React.createElement("div", {className: "rolled-up-version-summary"}, 
      React.createElement("span", {className: "version-header-time"}, formatted_time), 
      React.createElement("br", null), 
      React.createElement("a", {href: "/version/" + version.ids[i]}, commit), " - ", React.createElement("strong", null, author), 
      React.createElement("br", null), 
      message, 
      React.createElement("br", null)
    )
  )
};

// Grid

// The main class that binds to the root div. This contains all the distros, builds, and tasks
function Grid ({data, project, collapseInfo, buildVariantFilter, taskFilter}) {
  return (
    React.createElement("div", {className: "waterfall-grid"}, 
      
        data.rows.filter(function(row){
          return row.build_variant.display_name.toLowerCase().indexOf(buildVariantFilter.toLowerCase()) != -1;
        })
        .map(function(row){
          return React.createElement(Variant, {row: row, project: project, collapseInfo: collapseInfo, versions: data.versions, taskFilter: taskFilter, currentTime: data.current_time});
        })
      
    ) 
  )
};

function filterActiveTasks(tasks, activeStatuses){
  return _.filter(tasks, function(task) { 
      return _.contains(activeStatuses, task.status);
    });
}

// The class for each "row" of the waterfall page. Includes the build variant link, as well as the five columns
// of versions.
function Variant({row, versions, project, collapseInfo, taskFilter, currentTime}) {
      if (collapseInfo.collapsed){
        collapseInfo.noActive = _.every(row.builds, 
          function(build, versionId){
            var t = filterActiveTasks(build.tasks, collapseInfo.activeTaskStatuses);
            return t.length == 0;
          }) 
      }

      return (
      React.createElement("div", {className: "row variant-row"}, 
        React.createElement("div", {className: "col-xs-2 build-variants"}, 
          React.createElement("a", {href: "/build_variant/" + project + "/" + row.build_variant.id}, 
            row.build_variant.display_name
          )
        ), 
        React.createElement("div", {className: "col-xs-10"}, 
          React.createElement("div", {className: "row build-cells"}, 
            
              versions.map(function(version, i){
                return React.createElement(Build, {key: version.ids[0], 
                              build: row.builds[version.ids[0]], 
                              version: version, 
                              collapseInfo: collapseInfo, 
                              taskFilter: taskFilter, 
                              currentTime: currentTime})
              })
            
          )
        )
      )
    )
}


// Each Build class is one group of tasks for an version + build variant intersection
// We case on whether or not a build is active or not, and return either an ActiveBuild or InactiveBuild respectively

function Build({build, collapseInfo, version, taskFilter, currentTime}){
 
  // inactive build
  if (version.rolled_up) {
    return React.createElement(InactiveBuild, {className: "build"});
  }

  // no build for this version
  if (!build) {
    return React.createElement(EmptyBuild, null)  
  }


  // collapsed active build
  if (collapseInfo.collapsed) {
    if (collapseInfo.noActive){
      return (
      React.createElement("div", {className: "build"}, 
        React.createElement(CollapsedBuild, {build: build, activeTaskStatuses: collapseInfo.activeTaskStatuses})
      )
      )
    }
    // Can be modified to show combinations of tasks by statuses  
    var activeTasks = filterActiveTasks(build.tasks, collapseInfo.activeTaskStatuses)
    return (
      React.createElement("div", {className: "build"}, 
        React.createElement(CollapsedBuild, {build: build, activeTaskStatuses: collapseInfo.activeTaskStatuses}), 
        React.createElement(ActiveBuild, {tasks: activeTasks, currentTime: currentTime})
      )
    )
  } 
  // uncollapsed active build
  return (
    React.createElement("div", {className: "build"}, 
      React.createElement(ActiveBuild, {tasks: build.tasks, taskFilter: taskFilter, currentTime: currentTime})
    )
  )
}

// At least one task in the version is not inactive, so we display all build tasks with their appropiate colors signifying their status
function ActiveBuild({tasks, taskFilter, currentTime}){  

  if (taskFilter != null){
    tasks = _.filter(tasks, function(task){
      return task.display_name.toLowerCase().indexOf(taskFilter.toLowerCase()) != -1;
    });
  }

  return (
    React.createElement("div", {className: "active-build"}, 
      
        tasks.map(function(task){
          return React.createElement(Task, {task: task, currentTime: currentTime})
        })
      
    )
  )
}

// All tasks are inactive, so we display the words "inactive build"
function InactiveBuild ({}){
    return (React.createElement("div", {className: "inactive-build"}, " inactive build "))
}
// No build associated with a given version and variant, so we render an empty div
function EmptyBuild ({}){
    return (React.createElement("div", {className: "build"}, " "))
}

function TooltipContent({task, eta}) {
  var topLineContent = task.display_name + " - " + labelFromTask(task);
  if (task.status == 'success' || task.status == 'failed') {
    var dur = stringifyNanoseconds(task.time_taken);
    topLineContent += ' - ' + dur;
  }

  if (task.status !='failed' || !task.failed_test_names || task.failed_test_names.length == 0) {
    if (task.status == 'started') {
      return(
        React.createElement("span", {className: "waterfall-tooltip"}, 
          topLineContent, " - ", eta
        )
        )
    }
    return (
        React.createElement("span", {className: "waterfall-tooltip"}, 
          topLineContent
        )
        )
  }

  if (task.failed_test_names.length > MaxFailedTestDisplay) {
    return (
        React.createElement("span", {className: "waterfall-tooltip"}, 
          React.createElement("span", null, topLineContent), 
        React.createElement("div", {className: "header"}, 
          React.createElement("i", {className: "fa fa-times icon"}), 
          task.failed_test_names.length, " failed tests" 
          )
       )
        )
  }
  return(
      React.createElement("span", {className: "waterfall-tooltip"}, 
        React.createElement("span", null, topLineContent), 
      React.createElement("div", {className: "failed-tests"}, 
        
          task.failed_test_names.map(function(failed_test_name){
            return (
                React.createElement("div", null, 
                 React.createElement("i", {className: "fa fa-times icon"}), 
                  endOfPath(failed_test_name)
                )
                )
          })
        
        )
        )
      )
}

// CountdownClock is a class that manages decrementing duration every second.
// It takes as an argument nanosecondsRemaining and begins counting this number
// down as soon as it is instantiated.
class CountdownClock {
  constructor(nanosecondsRemaining) {
    this.tick = this.tick.bind(this);
    this.countdown = setInterval(this.tick, 1000);
    this.nanosecondsRemaining = nanosecondsRemaining;
  }
  tick() {
    this.nanosecondsRemaining -= 1 * (1000 * 1000 * 1000);
    if (this.nanosecondsRemaining <= 0) {
      this.nanosecondsRemaining = 0;
      clearInterval(this.countdown);
    }
  }
  getNanosecondsRemaining() {
    return this.nanosecondsRemaining;
  }
}

// ETADisplay is a react component that manages displaying a time being
// counted down. It takes as a prop a CountdownClock, which it uses to fetch
// the time left in the count down.
class ETADisplay extends React.Component {
  constructor(props) {
    super(props);
    this.tick = this.tick.bind(this);
    this.componentWillUnmount = this.componentWillUnmount.bind(this);

    this.update = setInterval(this.tick, 1000);
    this.countdownClock = this.props.countdownClock;

    var nsString = stringifyNanoseconds(this.countdownClock.getNanosecondsRemaining());

    if (this.countdownClock.getNanosecondsRemaining() <= 0) {
      nsString = 'unknown';
    }
    this.state = {
      ETAString: nsString
    };

  }

  tick() {
    var nsRemaining = this.countdownClock.getNanosecondsRemaining();
    var nsString = stringifyNanoseconds(nsRemaining);

    if (nsRemaining <= 0) {
      nsString = 'unknown';
      clearInterval(this.countdown);
    }
    this.setState({
      ETAString: nsString,
    });
  }

  componentWillUnmount() {
    clearInterval(this.interval);
  }
  render() {
    return (React.createElement("span", null, "ETA: ", this.state.ETAString));
  }
}


// A Task contains the information for a single task for a build, including the link to its page, and a tooltip
function Task({task, currentTime}) {
  var OverlayTrigger = ReactBootstrap.OverlayTrigger;
  var Popover = ReactBootstrap.Popover;
  var Tooltip = ReactBootstrap.Tooltip;
  var eta;
  if (task.status == 'started') {
    var timeRemaining = task.expected_duration - (currentTime - task.start_time);

    var clock = new CountdownClock(timeRemaining);
    var eta = (React.createElement(ETADisplay, {countdownClock: clock}));
  }
  var tooltip = (
      React.createElement(Tooltip, {id: "tooltip"}, 
        React.createElement(TooltipContent, {task: task, eta: eta})
      )
      )
  return (
    React.createElement(OverlayTrigger, {placement: "top", overlay: tooltip, animation: false}, 
      React.createElement("a", {href: "/task/" + task.id, className: "waterfall-box " + taskStatusClass(task)})
    )
  )
}

// A CollapsedBuild contains a set of PartialProgressBars, which in turn make up a full progress bar
// We iterate over the 5 different main types of task statuses, each of which have a different color association
function CollapsedBuild({build, activeTaskStatuses}){
  var taskStats = build.taskStatusCount;

  var taskTypes = {
    "success"      : taskStats.succeeded, 
    "dispatched"   : taskStats.started, 
    "system-failed": taskStats.timed_out,
    "undispatched" : taskStats.undispatched, 
    "inactive"     : taskStats.inactive,
    "failed"       : taskStats.failed,
  };

  // Remove all task summaries that have 0 tasks
  taskTypes = _.pick(taskTypes, function(count, status){
    return count > 0 && !(_.contains(activeTaskStatuses, status))
  });
  
  return (
    React.createElement("div", {className: "collapsed-build"}, 
      
        _.map(taskTypes, function(count, status) {
          return React.createElement(TaskSummary, {status: status, count: count, build: build});
        }) 
      
    )
  )
}

// A TaskSummary is the class for one rolled up task type
// A CollapsedBuild is comprised of an  array of contiguous TaskSummaries below individual failing tasks 
function TaskSummary({status, count, build}){
  var id_link = "/build/" + build.id;
  var OverlayTrigger = ReactBootstrap.OverlayTrigger;
  var Popover = ReactBootstrap.Popover;
  var Tooltip = ReactBootstrap.Tooltip;
  var tt = React.createElement(Tooltip, {id: "tooltip"}, count, " ", status);
  var classes = "task-summary " + status
  return (
    React.createElement(OverlayTrigger, {placement: "top", overlay: tt, animation: false}, 
      React.createElement("a", {href: id_link, className: classes}, 
        count
      )
    )
  )
}
