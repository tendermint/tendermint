#!/usr/bin/python

ANSIBLE_METADATA = {
    'metadata_version': '1.1',
    'status': ['preview'],
    'supported_by': 'community'
}

DOCUMENTATION = '''
---
module: tomlconfig

short_description: Ensure a particular configuration is added to a toml-formatted configuration file

version_added: "2.4"

description:
    - This module will add configuration to a toml-formatted configuration file.

options:
    dest:
        description:
            - The file to modify.
        required: true
        aliases: [ name, destfile ]
    json:
        description:
            - The configuration in json format to apply. Either C(json) or C(toml) has to be present.
        required: false
        default: '{}'
    toml:
        description:
            - The configuration in toml format to apply. Either C(json) or C(toml) has to be present.
        default: ''
    merge:
        description:
            - Used with C(state=present). If specified, it will merge the configuration. Othwerwise
              the configuration will be overwritten.
        required: false
        choices: [ "yes", "no" ]
        default: "yes"
    state:
        description:
            - Whether the configuration should be there or not.
        required: false
        choices: [ present, absent ]
        default: "present"
    create:
        description:
            - Used with C(state=present). If specified, the file will be created
              if it does not already exist. By default it will fail if the file
              is missing.
        required: false
        choices: [ "yes", "no" ]
        default: "no"
    backup:
        description:
            - Create a backup file including the timestamp information so you can
              get the original file back if you somehow clobbered it incorrectly.
        required: false
        choices: [ "yes", "no" ]
        default: "no"
    others:
        description:
            - All arguments accepted by the M(file) module also work here.
        required: false

extends_documentation_fragment:
    - files
    - validate

author:
    - "Greg Szabo (@greg-szabo)"
'''

EXAMPLES = '''
# Add a new section to a toml file
- name: Add comment section
  tomlconfig:
    dest: /etc/config.toml
    json: '{ "comment": { "comment1": "mycomment" } }'

# Rewrite a toml file with the configuration
- name: Create or overwrite config.toml
  tomlconfig:
    dest: /etc/config.toml
    json: '{ "regedit": { "freshfile": true } }'
    merge: no
    create: yes

- name: Set file permissions
  tomlconfig:
    dest: /etc/config.toml
    mode: 0600
    owner: root
    group: root
'''

RETURN = '''
config:
    description: The resultant configuration
    type: str
created:
    description: True if the file was freshly created
    type: bool
'''

from ansible.module_utils.basic import AnsibleModule
from ansible.module_utils.six import b
from ansible.module_utils._text import to_bytes, to_native
import tempfile
import toml as pytoml
import json
import copy
import os

def write_changes(module, b_lines, dest):

    tmpfd, tmpfile = tempfile.mkstemp()
    f = os.fdopen(tmpfd, 'wb')
    f.writelines(b_lines)
    f.close()

    validate = module.params.get('validate', None)
    valid = not validate
    if validate:
        if "%s" not in validate:
            module.fail_json(msg="validate must contain %%s: %s" % (validate))
        (rc, out, err) = module.run_command(to_bytes(validate % tmpfile, errors='surrogate_or_strict'))
        valid = rc == 0
        if rc != 0:
            module.fail_json(msg='failed to validate: '
                                 'rc:%s error:%s' % (rc, err))
    if valid:
        module.atomic_move(tmpfile,
                to_native(os.path.realpath(to_bytes(dest, errors='surrogate_or_strict')), errors='surrogate_or_strict'),
                unsafe_writes=module.params['unsafe_writes'])


def check_file_attrs(module, changed, message, diff):

    file_args = module.load_file_common_arguments(module.params)
    if module.set_fs_attributes_if_different(file_args, False, diff=diff):

        if changed:
            message += " and "
        changed = True
        message += "ownership, perms or SE linux context changed"

    return message, changed


#Merge dict d2 into dict d1 and return a new object
def deepmerge(d1, d2):
    if d1 is None:
        return copy.deepcopy(d2)
    if d2 is None:
        return copy.deepcopy(d1)
    if d1 == d2:
        return copy.deepcopy(d1)
    if isinstance(d1, dict) and isinstance(d2, dict):
        result={}
        for key in set(d1.keys()+d2.keys()):
            da = db = None
            if key in d1:
                da = d1[key]
            if key in d2:
                db = d2[key]
            result[key] = deepmerge(da, db)
        return result
    else:
        return copy.deepcopy(d2)


#Remove dict d2 from dict d1 and return a new object
def deepdiff(d1, d2):
    if d1 is None or d2 is None:
        return None
    if d1 == d2:
        return None
    if isinstance(d1, dict) and isinstance(d2, dict):
        result = {}
        for key in d1.keys():
            if key in d2:
                dd = deepdiff(d1[key],d2[key])
                if dd is not None:
                    result[key] = dd
            else:
                result[key] = d1[key]
        return result
    else:
        return None
    

def present(module, dest, conf, jsonbool, merge, create, backup):

    diff = {'before': '',
            'after': '',
            'before_header': '%s (content)' % dest,
            'after_header': '%s (content)' % dest}

    b_dest = to_bytes(dest, errors='surrogate_or_strict')
    if not os.path.exists(b_dest):
        if not create:
            module.fail_json(rc=257, msg='Destination %s does not exist !' % dest)
        b_destpath = os.path.dirname(b_dest)
        if not os.path.exists(b_destpath) and not module.check_mode:
            os.makedirs(b_destpath)
        b_lines = []
    else:
        f = open(b_dest, 'rb')
        b_lines = f.readlines()
        f.close()

    lines = to_native(b('').join(b_lines))

    if module._diff:
        diff['before'] = lines

    b_conf = to_bytes(conf, errors='surrogate_or_strict')

    tomlconfig = pytoml.loads(lines)
    config = {}
    if jsonbool:
        config = eval(b_conf)
    else:
        config = pytoml.loads(b_conf)

    if not isinstance(config, dict):
        if jsonbool:
            module.fail_json(msg="Invalid value in json parameter: {0}".format(config))
        else:
            module.fail_json(msg="Invalid value in toml parameter: {0}".format(config))

    b_lines_new = b_lines
    msg = ''
    changed = False

    if not merge:
        if tomlconfig != config:
            b_lines_new = to_bytes(pytoml.dumps(config))
            msg = 'config overwritten'
            changed = True
    else:
        mergedconfig = deepmerge(tomlconfig,config)
        if tomlconfig != mergedconfig:
            b_lines_new = to_bytes(pytoml.dumps(mergedconfig))
            msg = 'config merged'
            changed = True

    if module._diff:
        diff['after'] = to_native(b('').join(b_lines_new))

    backupdest = ""
    if changed and not module.check_mode:
        if backup and os.path.exists(b_dest):
            backupdest = module.backup_local(dest)
        write_changes(module, b_lines_new, dest)

    if module.check_mode and not os.path.exists(b_dest):
        module.exit_json(changed=changed, msg=msg, backup=backupdest, diff=diff)

    attr_diff = {}
    msg, changed = check_file_attrs(module, changed, msg, attr_diff)

    attr_diff['before_header'] = '%s (file attributes)' % dest
    attr_diff['after_header'] = '%s (file attributes)' % dest

    difflist = [diff, attr_diff]
    module.exit_json(changed=changed, msg=msg, backup=backupdest, diff=difflist)


def absent(module, dest, conf, jsonbool, backup):

    b_dest = to_bytes(dest, errors='surrogate_or_strict')
    if not os.path.exists(b_dest):
        module.exit_json(changed=False, msg="file not present")

    msg = ''
    diff = {'before': '',
            'after': '',
            'before_header': '%s (content)' % dest,
            'after_header': '%s (content)' % dest}

    f = open(b_dest, 'rb')
    b_lines = f.readlines()
    f.close()

    lines = to_native(b('').join(b_lines))
    b_conf = to_bytes(conf, errors='surrogate_or_strict')

    lines = to_native(b('').join(b_lines))
    tomlconfig = pytoml.loads(lines)
    config = {}
    if jsonbool:
        config = eval(b_conf)
    else:
        config = pytoml.loads(b_conf)

    if not isinstance(config, dict):
        if jsonbool:
            module.fail_json(msg="Invalid value in json parameter: {0}".format(config))
        else:
            module.fail_json(msg="Invalid value in toml parameter: {0}".format(config))

    if module._diff:
        diff['before'] = to_native(b('').join(b_lines))

    b_lines_new = b_lines
    msg = ''
    changed = False

    diffconfig = deepdiff(tomlconfig,config)
    if diffconfig is None:
        diffconfig = {}
    if tomlconfig != diffconfig:
        b_lines_new = to_bytes(pytoml.dumps(diffconfig))
        msg = 'config removed'
        changed = True

    if module._diff:
        diff['after'] = to_native(b('').join(b_lines_new))

    backupdest = ""
    if changed and not module.check_mode:
        if backup:
            backupdest = module.backup_local(dest)
        write_changes(module, b_lines_new, dest)

    attr_diff = {}
    msg, changed = check_file_attrs(module, changed, msg, attr_diff)

    attr_diff['before_header'] = '%s (file attributes)' % dest
    attr_diff['after_header'] = '%s (file attributes)' % dest

    difflist = [diff, attr_diff]

    module.exit_json(changed=changed, msg=msg, backup=backupdest, diff=difflist)


def main():

    # define the available arguments/parameters that a user can pass to
    # the module
    module_args = dict(
        dest=dict(type='str', required=True),
        json=dict(default=None),
        toml=dict(default=None),
        merge=dict(type='bool', default=True),
        state=dict(default='present', choices=['absent', 'present']),
        create=dict(type='bool', default=False),
        backup=dict(type='bool', default=False),
        validate=dict(default=None, type='str')
    )

    # the AnsibleModule object will be our abstraction working with Ansible
    # this includes instantiation, a couple of common attr would be the
    # args/params passed to the execution, as well as if the module
    # supports check mode
    module = AnsibleModule(
        argument_spec=module_args,
        mutually_exclusive=[['json', 'toml']],
        add_file_common_args=True,
        supports_check_mode=True
    )

    params = module.params
    create = params['create']
    merge = params['merge']
    backup = params['backup']
    dest = params['dest']

    b_dest = to_bytes(dest, errors='surrogate_or_strict')

    if os.path.isdir(b_dest):
        module.fail_json(rc=256, msg='Destination %s is a directory !' % dest)

    par_json, par_toml, jsonbool = params['json'], params['toml'], False
    if par_json is None:
       conf = par_toml
    else:
       conf = par_json
       jsonbool = True

    if params['state'] == 'present':
        present(module, dest, conf, jsonbool, merge, create, backup)
    else:
        absent(module, dest, conf, jsonbool, backup)
    
if __name__ == '__main__':
    main()

