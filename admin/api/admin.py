from django.contrib import admin

from .models import History, Location, Marriage, ParentChild, Person, Photo, Visitor


class LocationInline(admin.StackedInline):
    model = Location
    extra = 0


class PhotoInline(admin.TabularInline):
    model = Photo
    extra = 0
    fields = ['file_path', 'description', 'profile_pic', 'rotation']


class ParentLinkInline(admin.TabularInline):
    model = ParentChild
    fk_name = 'child'
    verbose_name = 'parent'
    verbose_name_plural = 'parents'
    autocomplete_fields = ['parent']
    extra = 0


class ChildLinkInline(admin.TabularInline):
    model = ParentChild
    fk_name = 'parent'
    verbose_name = 'child'
    verbose_name_plural = 'children'
    autocomplete_fields = ['child']
    extra = 0


@admin.register(Person)
class PersonAdmin(admin.ModelAdmin):
    list_display = ['name', 'birthyear', 'birthplace']
    search_fields = ['name', 'birthplace', 'bio']
    inlines = [LocationInline, ParentLinkInline, ChildLinkInline, PhotoInline]


@admin.register(ParentChild)
class ParentChildAdmin(admin.ModelAdmin):
    list_display = ['parent', 'child']
    search_fields = ['parent__name', 'child__name']
    autocomplete_fields = ['parent', 'child']


@admin.register(Marriage)
class MarriageAdmin(admin.ModelAdmin):
    list_display = ['person_a', 'person_b']
    search_fields = ['person_a__name', 'person_b__name']
    autocomplete_fields = ['person_a', 'person_b']


@admin.register(Location)
class LocationAdmin(admin.ModelAdmin):
    list_display = ['name', 'person', 'lat', 'lng']
    search_fields = ['name', 'person__name']


@admin.register(Photo)
class PhotoAdmin(admin.ModelAdmin):
    list_display = ['person', 'description', 'profile_pic', 'rotation']
    list_filter = ['profile_pic']
    search_fields = ['person__name', 'description']


@admin.register(History)
class HistoryAdmin(admin.ModelAdmin):
    list_display = ['created_at', 'username', 'action', 'recipient']
    list_filter = ['action']
    search_fields = ['username', 'recipient']


@admin.register(Visitor)
class VisitorAdmin(admin.ModelAdmin):
    list_display = ['ip_address', 'date']
    list_filter = ['date']
