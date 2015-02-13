import glob
import json
import os
import time
import urllib
import datetime

from os import listdir
from os.path import isfile
from os.path import join
from os.path import abspath
from os.path import exists

from glob import iglob
from flask import flash
from flask import redirect
from flask import render_template
from flask import request
from flask import url_for
from flask import send_file
from flask import make_response
from flask import Blueprint
from flask import jsonify

from application import app
from application import list_integers_string
from application import http_server_request
# from server import RENDER_PATH

# TODO: find a better way to fill/use this variable
BRENDER_SERVER = app.config['BRENDER_SERVER']


# Name of the Blueprint
jobs = Blueprint('jobs', __name__)


@jobs.route('/')
def index():
    jobs = http_server_request('get', '/jobs')
    jobs_list = []

    def seconds_to_time(seconds):
        return str(datetime.timedelta(seconds=seconds))

    for key, val in jobs.iteritems():

        remaining_time = val['remaining_time']
        if not remaining_time:
            remaining_time='-'
        else:
            remaining_time=seconds_to_time(remaining_time)
        average_time = val['average_time']
        if not average_time:
            average_time='-'
        else:
            average_time=seconds_to_time(average_time)
        total_time=val['total_time']
        if not total_time:
            total_time='-'
        else:
            total_time=seconds_to_time(total_time)
        job_time=val['job_time']
        if job_time:
            total_time="{0} ({1})".format(total_time, seconds_to_time(job_time))

        val['checkbox'] = '<input type="checkbox" value="' + key + '" />'
        jobs_list.append({
            "DT_RowId": "job_" + str(key),
            "0": val['checkbox'],
            "1": key,
            "2": val['job_name'],
            "3": val['percentage_done'],
            "4": val['status'],
            "5" : 'http://%s/jobs/thumbnails/%s' % (BRENDER_SERVER, key),
            "6" : remaining_time,
            "7" : average_time,
            "8" : total_time,
            "9" : val['activity'],
            "10" : 'asd',
            "11" : 'asd'
            })

    jobs_list = sorted(jobs_list, key=lambda x: x['1'])
    entries = json.dumps(jobs_list)

    return render_template('jobs/index.html', entries=entries, title='jobs')

@jobs.route('/<int:job_id>')
def job(job_id):
    print '[Debug] job_id is %s' % job_id
    #job = http_server_request('get', '/jobs/' + job_id)
    jobs = http_server_request('get', '/jobs')
    job = jobs[job_id]
    job['settings']=json.loads(job['settings'])

    #Tasks
    task_activity=''
    job['thumbnail']='http://%s/jobs/thumbnails/%s' % (BRENDER_SERVER, job_id)

    #job['thumb'] = last_thumbnail(job['id'])
    # render_dir = RENDER_PATH + "/" + str(job['id']) +  '/'
    # if exists(render_dir):
    #     job['render'] = map(lambda s : join("/" + render_dir, s), \
    #                     filter(lambda s : s.endswith(".thumb"), listdir(render_dir)))
    # else:
    #     job['render'] = '#'

    return render_template('jobs/view.html', job=job)


@jobs.route('/<int:job_id>.json')
def view_json(job_id):
    job = http_server_request('get', '/jobs/{0}'.format(job_id))
    return jsonify(job)


@jobs.route('/browse/', defaults={'path': ''})
@jobs.route('/browse/<path:path>',)
def jobs_browse(path):
    if len(path) > 0:
        path = os.path.join('/browse', path)
    else:
        path = "/browse"
    print path
    path_data = http_server_request('get', path)
    return render_template('browse_modal.html',
        # items=path_data['items'],
        items_list=path_data['items_list'],
        parent_folder=path_data['parent_path'])


@jobs.route('/delete', methods=['POST'])
def jobs_delete():
    job_ids = request.form['id']
    print(job_ids)
    params = {'id': job_ids}
    jobs = http_server_request('post', '/jobs/delete', params)
    return 'done'


@jobs.route('/update', methods=['POST'])
def jobs_update():
    command = request.form['command'].lower()
    job_ids = request.form['id']
    params = {'id': job_ids,
              'status' : command}
    if command in ['start', 'stop', 'respawn', 'reset']:
        jobs = http_server_request('put',
            '/jobs', params)
        return 'done'
    else:
        return 'error', 400


@jobs.route('/add', methods=['GET', 'POST'])
def add():
    if request.method == 'POST':
        job_values = {
            'project_id': request.form['project_id'],
            'job_name': request.form['job_name'],
            'frame_start': request.form['frame_start'],
            'frame_end': request.form['frame_end'],
            'chunk_size': request.form['chunk_size'],
            'current_frame': request.form['frame_start'],
            'filepath': request.form['filepath'],
            'render_settings': request.form['render_settings'],
            'format' : request.form['format'],
            'status': 'stopped',
            'priority': 10,
            'managers' : request.form.getlist('managers'),
            'job_type' : request.form['job_type'],
            'owner': 'fsiddi'
        }

        http_server_request('post', '/jobs', job_values)

        #  flashing does not work because we use redirect_url
        #  flash('New job added!')

        return redirect(url_for('jobs.index'))
    else:
        render_settings = http_server_request('get', '/settings/render')
        projects = http_server_request('get', '/projects')
        settings = http_server_request('get', '/settings')
        managers = http_server_request('get', '/managers')
        return render_template('jobs/add_modal.html',
                            render_settings=render_settings,
                            settings=settings,
                            projects=projects,
                            managers=filter(lambda m : m['connection'] == 'online',
                                            managers.values()))

