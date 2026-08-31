from collections import defaultdict

from django.db import migrations


def backfill(apps, schema_editor):
    Person = apps.get_model('api', 'Person')
    ParentChild = apps.get_model('api', 'ParentChild')
    Marriage = apps.get_model('api', 'Marriage')

    known = set(Person.objects.values_list('id', flat=True))
    parents, spouses, siblings = set(), set(), set()

    for person_id, relations in Person.objects.values_list('id', 'relations'):
        for relation in relations or []:
            other = relation['id']
            if other == person_id or other not in known:
                continue

            kind = relation['relation']
            if kind == 'Parent':
                parents.add((other, person_id))
            elif kind == 'Child':
                parents.add((person_id, other))
            elif kind == 'Spouse':
                spouses.add((min(person_id, other), max(person_id, other)))
            elif kind == 'Sibling':
                siblings.add((min(person_id, other), max(person_id, other)))

    of_child = defaultdict(set)
    for parent, child in parents:
        of_child[child].add(parent)

    unshared = sorted(pair for pair in siblings if not of_child[pair[0]] & of_child[pair[1]])
    if unshared:
        raise RuntimeError(f'sibling pairs with no shared parent, would be lost: {unshared}')

    ParentChild.objects.bulk_create(ParentChild(parent_id=p, child_id=c) for p, c in sorted(parents))
    Marriage.objects.bulk_create(Marriage(person_a_id=a, person_b_id=b) for a, b in sorted(spouses))


def clear(apps, schema_editor):
    apps.get_model('api', 'ParentChild').objects.all().delete()
    apps.get_model('api', 'Marriage').objects.all().delete()


class Migration(migrations.Migration):

    dependencies = [
        ('api', '0008_parentchild_marriage'),
    ]

    operations = [
        migrations.RunPython(backfill, clear),
    ]
