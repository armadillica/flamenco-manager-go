function load_workers() {
    var status_to_bootstrap_class = {
        'awake': 'success',
        'down': 'default',
        'timeout': 'danger',
    }

    $.get('/as-json')
    .done(function(info) {
        // Split workers into "current" and "idle for too long"
        var too_long = 14 * 24 * 3600000; // in milliseconds
        var current_workers = [];
        var idle_workers = [];

        $('#managerversion').text(info.version);

        // Make sure we can iterate over the list.
        if (typeof info.workers == 'undefined' || info.workers == null) {
            info.workers = [];
        }

        for (worker of info.workers) {
            var as_date = new Date(worker.last_activity);
            var timediff = Date.now() - as_date;  // in milliseconds
            if (typeof worker.last_activity == 'undefined' || timediff > too_long) {
                idle_workers.push(worker);
            } else {
                current_workers.push(worker);
            }
        }

        // Construct the overall status view.
        var $section = $('#status');
        $section.html('');

        var $dl = $('<dl>').addClass('dl-horizontal');
        $dl.append($('<dt>').text('Nr. of workers'));
        $dl.append($('<dd>').text(info.nr_of_workers));
        $dl.append($('<dt>')
            .text('Nr. of tasks')
            .attr('title', 'Number of tasks in database. Probably not all queued.'));
        $dl.append($('<dd>').text(info.nr_of_tasks));
        $dl.append($('<dt>').text('Server'));
        $dd = $('<dd>');
        $dd.append($('<a>').attr('href', info.server + 'flamenco/').text(info.server));
        $dl.append($dd);
        if (idle_workers.length > 0) {
            $dl.append($('<dt>')
                .text('Old workers')
                .attr('title', 'Workers not seen in over a month.'));
            $dd = $('<dd>');
            for (worker of idle_workers) {
                $dd.append($('<span>')
                    .addClass('idle-worker-name')
                    .text(worker.nickname)
                    .attr('title', worker._id)
                );
            }
            $dl.append($dd);
        }
        $section.append($dl);

        // Construct the worker list.
        var $section = $('#workers');
        $section.html('');

        for (worker of current_workers) {
            var $row = $('<tr>')
                .attr('id', worker._id)
                .addClass("status-" + worker.status);

            var status_class = status_to_bootstrap_class[worker.status];
            if (typeof status_class != 'undefined') $row.addClass(status_class);

            $row.append($('<td>').text(worker.nickname));
            $row.append($('<td>').text(worker._id));
            $row.append($('<td>').text(worker.address));
            $row.append($('<td>').text(worker.status || '-none-').addClass('status-' + worker.status));

            var software = '-unknown-';
            if (worker.software) {
                /* 'Flamenco-Worker' is the default software, so don't mention that;
                 * do keep the version number, though. */
                software = worker.software.replace('Flamenco-Worker/', '');
            }
            $row.append($('<td>').text(software));
            // $row.append($('<td>').text(worker.platform));

            $task_td = $('<td>');
            if (typeof worker.current_task != 'undefined') {
                var task_text;
                if (typeof worker.current_task_status != 'undefined') {
                    if (typeof worker.current_task_updated != 'undefined') {
                        task_text = worker.current_task + " (" + worker.current_task_status + " " + time_diff(worker.current_task_updated) + ")";
                    } else {
                        task_text = worker.current_task + " (" + worker.current_task_status + ")";
                    }
                } else {
                    task_text = worker.current_task;
                }
                $tasklink = $('<a>')
                    .attr('href', info.server + 'flamenco/tasks/' + worker.current_task)
                    .text(task_text);
                $task_td.append($tasklink);
            } else {
                $task_td.text('-none-');
            }
            $row.append($task_td);

            $row.append($('<td>')
                .text(time_diff(worker.last_activity))
                .attr('title', new Date(worker.last_activity)));
            // $dl.append($('<dd>').text(worker.supported_job_types.join(', ')));

            $section.append($row);
        }

        // Everything went fine, let's try it again soon.
        setTimeout(load_workers, 2000);
    })
    .fail(function(error) {
        var $section = $('#status');
        var $p = $('<p>').addClass('error');

        if (error.status) {
            $p.text('Error ' + error.status + ': ' + error.responseText);
        } else {
            $p.text('Unable to get the status report. Is the Manager still running & reachable?');
        }
        $section.html($p);

        // Everything is bugging out, let's try it again soon-ish.
        setTimeout(load_workers, 10000);
    });
}

$(load_workers);


function time_diff(timestamp) {
    if (typeof timestamp == 'undefined') {
        return '-nevah-';
    }

    var as_date = new Date(timestamp);
    var timediff = Date.now() - as_date;  // in milliseconds

    if (timediff < 1000) {
        return 'just now';
    }
    if (timediff < 60000) {  // less than a minute
        return Math.round(timediff / 1000) + ' seconds ago';
    }
    if (timediff < 3600000) { // less than an hour
        return Math.round(timediff / 60000) + ' minutes ago';
    }
    if (timediff < 24 * 3600000) { // less than a day hour
        return Math.round(timediff / 3600000) + ' hours ago';
    }

    return as_date;
}

function downloadkick() {
    var button = $(this);
    button.fadeOut();
    $.get('/kick')
    .done(function() {
        button.fadeIn();
    })
    .fail(function(err) {
        button.text('Error, see console.');
        button.fadeIn();
        console.log(err);
    });
}

$(function() {
    $('#downloadkick').on('click', downloadkick);
})
