from django.conf import settings
from django.db import migrations

# Prod person ids belonging to Elya. Helen owns everyone else, plus id 1, which
# the two of them share.
ELYA_PERSON_IDS = [1, 6, 7, 8, 16, 17, 18, 19, 20, 21, 22, 32, 33, 34]
SHARED_PERSON_IDS = [1]

OWNER_USERNAMES = ('Elya', 'Helen')


def backfill(apps, schema_editor):
    """Assign prod's existing people to their owners.

    Keyed off usernames rather than user ids so this is a no-op anywhere those
    accounts do not exist -- local dev has admin and guest, so the fixtures are
    left alone and their people stay unowned.

    ignore_conflicts leans on the through table's unique constraint, so running
    this against rows that are already assigned adds nothing.
    """
    Person = apps.get_model('api', 'Person')
    User = apps.get_model(settings.AUTH_USER_MODEL)
    Ownership = Person.owners.through

    users = {u.username: u.pk for u in User.objects.filter(username__in=OWNER_USERNAMES)}

    elya = users.get('Elya')
    helen = users.get('Helen')
    if elya is None or helen is None:
        return

    person_ids = set(Person.objects.values_list('pk', flat=True))

    elya_ids = person_ids & set(ELYA_PERSON_IDS)
    helen_ids = (person_ids - elya_ids) | (person_ids & set(SHARED_PERSON_IDS))

    Ownership.objects.bulk_create(
        [Ownership(person_id=pk, user_id=elya) for pk in sorted(elya_ids)]
        + [Ownership(person_id=pk, user_id=helen) for pk in sorted(helen_ids)],
        ignore_conflicts=True,
    )


def clear(apps, schema_editor):
    Person = apps.get_model('api', 'Person')
    User = apps.get_model(settings.AUTH_USER_MODEL)
    Ownership = Person.owners.through

    owner_ids = User.objects.filter(username__in=OWNER_USERNAMES).values_list('pk', flat=True)

    Ownership.objects.filter(user_id__in=owner_ids).delete()


class Migration(migrations.Migration):

    dependencies = [
        ('api', '0012_person_owners'),
    ]

    operations = [
        migrations.RunPython(backfill, clear),
    ]
